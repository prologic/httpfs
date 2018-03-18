package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"github.com/prologic/httpfs"
	"github.com/prologic/httpfs/fsapi"
)

func debugLog(msg interface{}) {
	fmt.Printf("%s\n", msg)
}

func main() {
	var (
		version   bool
		debug     bool
		url       string
		tlsverify bool
		mount     string
	)

	flag.BoolVar(&version, "v", false, "display version information")

	flag.BoolVar(&debug, "debug", false, "enable debug log messages to stderr")
	flag.StringVar(&url, "url", "", "url of httpsfs backend (required)")
	flag.BoolVar(&tlsverify, "tlsverify", false, "enable TLS verification")
	flag.StringVar(&mount, "mount", "", "path to mount volume (required)")

	flag.Parse()

	if version {
		fmt.Printf(httpfs.FullVersion())
		os.Exit(0)
	}

	if mount == "" || url == "" {
		fmt.Println("Both -mount and -url are required")
		os.Exit(2)
	}

	c, err := fuse.Mount(
		mount,
		fuse.FSName("httpfs"),
		fuse.Subtype("httpfs"),
		fuse.VolumeName("HTTP FS"),
		// fuse.LocalVolume(),
		fuse.AllowOther(),

		fuse.MaxReadahead(2^20),
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cfg := &fs.Config{}
	if debug {
		cfg.Debug = debugLog
	}
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
