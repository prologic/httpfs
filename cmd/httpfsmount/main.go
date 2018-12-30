package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/prologic/httpfs"
	"github.com/prologic/httpfs/fsapi"
)

var (
	version   bool
	debug     bool
	url       string
	tlsverify bool
	mount     string
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <mount>\n", path.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.BoolVar(&version, "v", false, "display version information")

	flag.BoolVar(&debug, "d", false, "enable debug log messages to stderr")
	flag.StringVar(&url, "u", "http://localhost:8000", "url of httpsfs backend")
	flag.BoolVar(&tlsverify, "tlsverify", false, "enable TLS verification")

	flag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
		fuse.Debug = func(msg interface{}) {
			log.Debugf("%s\n", msg)
		}
	} else {
		log.SetLevel(log.InfoLevel)
	}

	if version {
		fmt.Printf(httpfs.FullVersion())
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	mount = flag.Arg(0)

	c, err := fuse.Mount(
		mount,
		fuse.FSName("httpfs"),
		fuse.Subtype("httpfs"),
		fuse.VolumeName("HTTP FS"),
		fuse.AllowOther(),

		fuse.MaxReadahead(128*1024),
		fuse.WritebackCache(),
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
		fuse.AsyncRead(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cfg := &fs.Config{}
	srv := fs.New(c, cfg)
	filesys := fsapi.NewHTTPFS(url, tlsverify)

	if err := srv.Serve(filesys); err != nil {
		log.Fatal(err)
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
