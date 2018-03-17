package fsapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"

	httpfstypes "github.com/prologic/httpfs/types"

	"bazil.org/fuse"
)

func prepareTimeString(ts string) string {
	return strings.Trim(strings.Join(strings.SplitN(
		strings.Trim(ts, "\t "), " ", 3)[0:2], " "), "\r\n\t ")
}

// ParseTime ...
func ParseTime(n *html.Node) (t time.Time) {
	if ts := prepareTimeString(n.Data); ts != "" {
		var err error
		t, err = time.Parse("2-Jan-2006 15:04", ts)
		if err != nil {
			log.Printf("ParseTime('%s'): %s", ts, err)
		}
	}
	return
}

// ErrorFromStatus ...
func ErrorFromStatus(code int) fuse.Errno {
	switch code {
	case 404:
		return fuse.ENOENT
	case 403:
		return fuse.EPERM
	default:
		return fuse.EIO
	}
}

type fileStat struct {
	name  string
	size  int64
	mode  uint32
	mtime int64
	isdir bool
}

func (fs fileStat) Name() string {
	return fs.name
}

func (fs fileStat) Size() int64 {
	return fs.size
}

func (fs fileStat) Mode() os.FileMode {
	return os.FileMode(fs.mode)
}

func (fs fileStat) ModTime() time.Time {
	return time.Unix(fs.mtime, 0)
}

func (fs fileStat) IsDir() bool {
	return fs.isdir
}

func (fs fileStat) Sys() interface{} {
	return nil
}

// Client ...
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient ...
func NewClient(url string, tlsverify bool) *Client {
	if strings.HasPrefix(url, "https://") {
		return &Client{
			baseURL: url,
			client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: !tlsverify},
				},
			},
		}
	}

	return &Client{
		baseURL: url,
		client:  &http.Client{},
	}
}

// NewRequest ...
func (c Client) NewRequest(method, path string, body io.Reader) *http.Request {
	//log.Printf("client.NewRequest(%s, %s)\n", method, path)
	req, _ := http.NewRequest(method, c.baseURL+path, body)
	return req
}

// Head ...
func (c Client) Head(path string) *http.Request {
	return c.NewRequest("HEAD", path, nil)
}

// Get ...
func (c Client) Get(path string) *http.Request {
	return c.NewRequest("GET", path, nil)
}

// Put ...
func (c Client) Put(path string, body io.Reader) *http.Request {
	return c.NewRequest("PUT", path, body)
}

// Stat ...
func (c Client) Stat(path string) (os.FileInfo, error) {
	//log.Printf("client.Stat(%s)\n", path)
	r, e := c.client.Do(c.Head(path))
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return nil, e
	}

	//log.Printf(" status=%d\n", r.StatusCode)

	if r.StatusCode != http.StatusOK {
		return nil, ErrorFromStatus(r.StatusCode)
	}

	var mtime int64

	t, err := http.ParseTime(r.Header.Get("Last-Modified"))
	if err != nil {
		mtime = 0
		//log.Printf(" E: error prasing Last-Modified: %s\n", err)
	} else {
		mtime = t.Unix()
	}

	size := SafeParseInt64(r.Header.Get("Content-Length"))
	mode := uint32(SafeParseInt64(r.Header.Get("X-File-Mode")))
	isdir := SafeParseBool(r.Header.Get("X-Is-Dir"))

	//log.Printf(" size=%d mtime=%d mode=%d isdir=%b\n", size, mtime, mode, isdir,)

	return fileStat{
		name:  path,
		size:  size,
		mode:  mode,
		mtime: mtime,
		isdir: isdir,
	}, nil
}

// Readdir ...
func (c Client) Readdir(path string) ([]os.FileInfo, error) {
	var (
		out     []os.FileInfo
		entries []httpfstypes.Entry
	)

	r, e := c.client.Do(c.Get(path))
	defer r.Body.Close()

	if e != nil {
		return nil, e
	}

	if r.StatusCode != http.StatusOK {
		return nil, ErrorFromStatus(r.StatusCode)
	}

	switch strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0] {
	case "text/html":
		doc, err := html.Parse(r.Body)
		if err != nil {
			return nil, err
		}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						name, err := url.QueryUnescape(a.Val)
						if err != nil {
							log.Printf("url.QueryUnescape(%s): %s", a.Val, err)
						}
						name = strings.TrimRight(name, "/")
						entries = append(entries, httpfstypes.Entry{
							Name:    name,
							IsDir:   a.Val[len(a.Val)-1] == '/',
							ModTime: 0, // ParseTime(n.NextSibling).Unix(),
						})
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
	case "application/json":
		data, _ := ioutil.ReadAll(r.Body)
		if err := json.Unmarshal(data, &entries); nil != err {
			//log.Printf("Error: %s\n", err)
			return nil, err
		}
	default:
		panic("Unsupported directory response.")
	}

	for _, entry := range entries {
		out = append(out, fileStat{
			name:  entry.Name,
			size:  entry.Size,
			mode:  entry.Mode,
			mtime: entry.ModTime,
			isdir: entry.IsDir,
		})
	}

	return out, nil
}

// Mkdir ...
func (c Client) Mkdir(path string, perm os.FileMode) error {
	//log.Printf("client.Mkdir(%q, %d)\n", path, perm)

	req := c.NewRequest("MKDIR", path, nil)

	q := req.URL.Query()
	q.Add("mode", fmt.Sprintf("%d", perm))
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Link ...
func (c Client) Link(path, name string) error {
	//log.Printf("client.Link(%s, %s)\n", path, name)

	req := c.NewRequest("LINK", path, nil)

	q := req.URL.Query()
	q.Add("name", name)
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Symlink ...
func (c Client) Symlink(target, name string) error {
	//log.Printf("client.Symlink(%s, %s)\n", target, name)

	req := c.NewRequest("LINK", target, nil)

	q := req.URL.Query()
	q.Add("name", name)
	q.Add("soft", "1")
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Rename ...
func (c Client) Rename(oldpath, newpath string) error {
	//log.Printf("client.Rename(%s, %s)\n", oldpath, newpath)

	req := c.NewRequest("RENAME", oldpath, nil)

	q := req.URL.Query()
	q.Add("name", newpath)
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Delete ...
func (c Client) Delete(path string) error {
	//log.Printf("client.Delete(%s)\n", path)

	req := c.NewRequest("DELETE", path, nil)
	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Chmod ...
func (c Client) Chmod(path string, mode os.FileMode) error {
	//log.Printf("client.Chmod(%s, %d)\n", path, int(mode))

	req := c.NewRequest("CHMOD", path, nil)

	q := req.URL.Query()
	q.Add("mode", fmt.Sprintf("%d", mode))
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

// Truncate ...
func (c Client) Truncate(path string, size uint64) error {
	//log.Printf("client.Truncate(%s, %d)\n", path, size)

	req := c.NewRequest("TRUNCATE", path, nil)

	q := req.URL.Query()
	q.Add("size", fmt.Sprintf("%d", size))
	req.URL.RawQuery = q.Encode()

	r, e := c.client.Do(req)
	if e != nil {
		//log.Printf(" E: %s\n", e)
		return e
	}

	if r.StatusCode != http.StatusOK {
		return ErrorFromStatus(r.StatusCode)
	}

	return nil
}

/*
// OpenFile ...
func (c Client) OpenFile(path string, flags int, perm os.FileMode) (Handle, error) {
	return Handle{
		path:  path,
		flags: flags,
		perm:  perm,

		client: c,
	}, nil
}
*/
