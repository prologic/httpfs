package fsapi

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"

	"bazil.org/fuse"
)

// Handle ...
type Handle struct {
	f     *File
	path  string
	flags int
	perm  os.FileMode

	client *Client
}

// Close ...
func (h Handle) Close() error {
	return nil
}

// ReadAt ...
func (h Handle) ReadAt(buf []byte, offset int64) (int, error) {
	log.Debugf("handle.ReadAt(%s, %d)\n", h.path, offset)

	req := h.client.Get(h.path)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))

	r, err := h.client.client.Do(req)
	if err != nil {
		log.Debugf(" E: %s\n", err)
		return 0, fuse.EIO
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusPartialContent {
		log.Debugf(" status=%d\n", r.StatusCode)
		return 0, ErrorFromStatus(r.StatusCode)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, fuse.EIO
	}

	copy(buf, data)

	return len(buf), nil
}

// WriteAt ...
func (h Handle) WriteAt(buf []byte, flags int, offset int64) (int, error) {
	log.Debugf("handle.WriteAt(%s, %d, %d)\n", h.path, flags, offset)

	log.Debugf(" flags=%d\n", flags)
	log.Debugf(" perm=%d\n", h.perm)
	log.Debugf(" offset=%d\n", offset)
	log.Debugf(" len(buf)=%d\n", len(buf))

	if h.f.created {
		h.f.created = false
		flags |= os.O_CREATE | os.O_EXCL
	}

	body := bytes.NewReader(buf)

	req := h.client.Put(h.path, body)

	q := req.URL.Query()
	q.Add("flags", fmt.Sprintf("%d", flags))
	q.Add("perm", fmt.Sprintf("%d", h.perm))
	q.Add("offset", fmt.Sprintf("%d", offset))
	req.URL.RawQuery = q.Encode()

	r, err := h.client.client.Do(req)
	defer r.Body.Close()
	if err != nil {
		log.Debugf(" E: %s\n", err)
		return 0, fuse.EIO
	}

	if r.StatusCode == http.StatusOK {
		return len(buf), nil
	} else if r.StatusCode != http.StatusPartialContent {
		log.Debugf(" status=%d\n", r.StatusCode)
		return 0, ErrorFromStatus(r.StatusCode)
	}

	b, e := ioutil.ReadAll(r.Body)
	if e != nil {
		log.Debugf(" E: error reading body: %s\n", e)
		return 0, e
	}

	n, e := strconv.ParseInt(string(b), 10, 32)
	if e != nil {
		log.Debugf(" E: error parsing body: %s\n", e)
		return 0, e
	}

	return int(n), nil
}
