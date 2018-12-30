package fsapi

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

var _ fs.Node = (*Dir)(nil)
var _ fs.NodeCreater = (*Dir)(nil)
var _ fs.NodeMkdirer = (*Dir)(nil)
var _ fs.NodeRemover = (*Dir)(nil)
var _ fs.NodeRenamer = (*Dir)(nil)
var _ fs.NodeLinker = (*Dir)(nil)
var _ fs.NodeSymlinker = (*Dir)(nil)
var _ fs.NodeStringLookuper = (*Dir)(nil)

// Dir ...
type Dir struct {
	sync.RWMutex
	attr fuse.Attr

	path   string
	fs     *HTTPFS
	parent *Dir
}

// Attr ...
func (d *Dir) Attr(ctx context.Context, o *fuse.Attr) error {
	d.RLock()
	*o = d.attr
	d.RUnlock()
	return nil
}

var ignoreNames = map[string]struct{}{
	"DCIM":                                {},
	"Backups.backupdb":                    {},
	".Spotlight-V100":                     {},
	"mach_kernel":                         {},
	".metadata_never_index":               {},
	".metadata_never_index_unless_rootfs": {},
	".DS_Store":                           {},
	".localized":                          {},
	".hidden":                             {},
	"._.":                                 {},
}

// Lookup ...
func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if _, ignore := ignoreNames[name]; !ignore && !strings.HasPrefix(name, "._") {
		log.Debugf("dir.Lookup(%s)\n", name)

		d.RLock()
		defer d.RUnlock()

		path := filepath.Join(d.path, name)
		log.Debugf(" path=%s\n", path)
		stats, err := d.fs.client.Stat(path)
		if err != nil {
			log.Debugf(" E: %s\n", err)
			return nil, fuse.ENOENT
		}

		switch {
		case stats.IsDir():
			log.Debugf(" -> Directory\n")
			return d.fs.newDir(path, stats.Mode()), nil
		case stats.Mode()&os.ModeSymlink == os.ModeSymlink:
			log.Debugf(" -> Symlink\n")
			return d.fs.newFile(path, stats.Mode()), nil
		case stats.Mode().IsRegular():
			log.Debugf(" -> File\n")
			return d.fs.newFile(path, stats.Mode()), nil
		default:
			panic("Unknown type in filesystem")
		}
	}

	return nil, fuse.ENOENT
}

// ReadDirAll ...
func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	log.Debugf("dir.ReadDirAll(%s)\n", d.path)

	d.RLock()
	var out []fuse.Dirent

	files, err := d.fs.client.Readdir(d.path)
	if err != nil {
		log.Debugf(" E: %s\n", err)
		return nil, err
	}

	for _, node := range files {
		de := fuse.Dirent{Name: node.Name()}
		if node.IsDir() {
			de.Type = fuse.DT_Dir
		} else if node.Mode()&os.ModeSymlink == os.ModeSymlink {
			de.Type = fuse.DT_Link
		} else if node.Mode().IsRegular() {
			de.Type = fuse.DT_File
		}
		log.Debugf(" %+v\n", de)
		out = append(out, de)
	}

	d.RUnlock()
	return out, nil
}

// Mkdir ...
func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	d.Lock()
	defer d.Unlock()
	log.Debugf(" req=%+v\n", req)
	if exists := d.exists(req.Name); exists {
		//log.Println(" E: directory already exists")
		return nil, fuse.EEXIST
	}

	path := filepath.Join(d.path, req.Name)
	n := d.fs.newDir(path, req.Mode)

	if err := d.fs.client.Mkdir(path, req.Mode); err != nil {
		log.Debugf(" E: %s\n", err)
		return nil, err
	}
	return n, nil
}

// Create ...
func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	log.Debugf("dir.Create(%s)\n", req.Name)

	d.Lock()
	defer d.Unlock()
	if exists := d.exists(req.Name); exists {
		//log.Println(" E: file or directory already exists!")
		return nil, nil, fuse.EEXIST
	}

	path := filepath.Join(d.path, req.Name)

	f := d.fs.newFile(path, req.Mode)
	f.created = true
	f.fs = d.fs

	handle := Handle{
		f:     f,
		path:  path,
		flags: int(req.Flags),
		perm:  req.Mode,

		client: d.fs.client,
	}

	f.handle = &handle

	resp.Attr = f.attr

	return f, f, nil
}

// Link ...
func (d *Dir) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (newNode fs.Node, err error) {
	log.Debugf("dir.Link(%q, %q)\n", d.path, req.NewName)

	nd := newNode.(*Dir)

	log.Debugf(" req=%+v\n", req)

	if d.attr.Inode == nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
	} else if d.attr.Inode < nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
		nd.Lock()
		defer nd.Unlock()
	} else {
		nd.Lock()
		defer nd.Unlock()
		d.Lock()
		defer d.Unlock()
	}

	if exists := d.exists(req.NewName); !exists {
		log.Debugf(" E: link exists\n")
		return nil, fuse.ENOENT
	}

	newPath := filepath.Join(nd.path, req.NewName)

	if err := d.fs.client.Link(d.path, newPath); err != nil {
		log.Debugf(" E: %s\n", err)
		return nil, err
	}

	return nd, nil
}

// Symlink ...
func (d *Dir) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	log.Debugf("dir.Symlink(%q, %q)\n", req.Target, req.NewName)

	nd := d
	nd.attr.Mode |= os.ModeSymlink

	log.Debugf(" req=%+v\n", req)

	if d.attr.Inode == nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
	} else if d.attr.Inode < nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
		nd.Lock()
		defer nd.Unlock()
	} else {
		nd.Lock()
		defer nd.Unlock()
		d.Lock()
		defer d.Unlock()
	}

	if exists := d.exists(req.NewName); exists {
		log.Debugf(" E: link exists\n")
		return nil, fuse.ENOENT
	}

	targetPath := filepath.Join(d.path, req.Target)
	newPath := filepath.Join(nd.path, req.NewName)

	if err := d.fs.client.Symlink(targetPath, newPath); err != nil {
		log.Debugf(" E: %s\n", err)
		return nil, err
	}

	return nd, nil
}

// Rename ...
func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	log.Debugf("dir.Rename(%s, %s)\n", req.OldName, req.NewName)

	nd := newDir.(*Dir)

	log.Debugf(" req=%+v\n", req)

	if d.attr.Inode == nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
	} else if d.attr.Inode < nd.attr.Inode {
		d.Lock()
		defer d.Unlock()
		nd.Lock()
		defer nd.Unlock()
	} else {
		nd.Lock()
		defer nd.Unlock()
		d.Lock()
		defer d.Unlock()
	}

	if exists := d.exists(req.OldName); !exists {
		//log.Println(" E: no such file or directory")
		return fuse.ENOENT
	}

	oldPath := filepath.Join(d.path, req.OldName)
	newPath := filepath.Join(nd.path, req.NewName)

	if err := d.fs.client.Rename(oldPath, newPath); err != nil {
		log.Debugf(" E: %s\n", err)
		return err
	}

	return nil
}

// Remove ...
func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	log.Debugf("dir.Remove(%s)\n", req.Name)

	d.Lock()
	defer d.Unlock()

	log.Debugf(" req=%q\n", req)

	if exists := d.exists(req.Name); !exists {
		log.Debugf(" E: no such flie or directory\n")
		return fuse.ENOENT
	}
	// else if hasChildren() {
	// 	//log.Println("Remove ERR: ENOENT")
	// 	return fuse.ENOENT
	// }

	path := filepath.Join(d.path, req.Name)
	if err := d.fs.client.Delete(path); err != nil {
		log.Debugf(" E: %s\n", err)
		return err
	}

	return nil
}

func (d *Dir) exists(name string) bool {
	path := filepath.Join(d.path, name)
	_, err := d.fs.client.Stat(path)
	if err != nil {
		return false
	}

	return true
}
