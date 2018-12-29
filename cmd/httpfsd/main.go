package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	"github.com/prologic/httpfs"
	"github.com/prologic/httpfs/webapi"
	"github.com/unrolled/logger"
)

var (
	version  bool
	tls      bool
	tlscert  string
	tlskey   string
	readonly bool
	debug    bool
	bind     string
	root     string
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <root>\n", path.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.BoolVar(&version, "v", false, "display version information")
	flag.BoolVar(&debug, "d", false, "set debug logging")

	flag.StringVar(&bind, "b", "0.0.0.0:8000", "[int]:<port> to bind to")
	flag.BoolVar(&readonly, "r", false, "set read-only mode")

	flag.BoolVar(&tls, "tls", false, "Use TLS")
	flag.StringVar(&tlscert, "tlscert", "server.crt", "server certificate")
	flag.StringVar(&tlskey, "tlskey", "server.key", "server key")
}

func main() {
	flag.Parse()

	if debug {
		log.SetLevel(log.DebugLevel)
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

	root = flag.Arg(0)

	os.MkdirAll(root, 0700)

	http.Handle("/", webapi.FileServer(root, readonly))

	var handler http.Handler

	handler = logger.New(logger.Options{
		Prefix:               "httpfsd",
		RemoteAddressHeaders: []string{"X-Forwarded-For"},
	}).Handler(http.DefaultServeMux)

	if tls {
		log.Fatal(
			http.ListenAndServeTLS(
				bind,
				tlscert,
				tlskey,
				handler,
			),
		)
	} else {
		log.Fatal(http.ListenAndServe(bind, handler))
	}
}
