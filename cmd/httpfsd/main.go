package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/namsral/flag"

	"github.com/prologic/httpfs"
	"github.com/prologic/httpfs/webapi"
)

func cwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return cwd
}

// Log ...
func Log(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {
	var (
		version  bool
		config   string
		tls      bool
		tlscert  string
		tlskey   string
		readonly bool
		debug    bool
		bind     string
		root     string
	)

	flag.BoolVar(&version, "v", false, "display version information")

	flag.StringVar(&config, "config", "", "config file")
	flag.BoolVar(&tls, "tls", false, "Use TLS")
	flag.BoolVar(&debug, "debug", false, "set debug logging")
	flag.BoolVar(&readonly, "readonly", false, "set read-only mode")
	flag.StringVar(&tlscert, "tlscert", "server.crt", "server certificate")
	flag.StringVar(&tlskey, "tlskey", "server.key", "server key")
	flag.StringVar(&bind, "bind", "0.0.0.0:8000", "[int]:<port> to bind to")
	flag.StringVar(&root, "root", cwd(), "path to serve")

	flag.Parse()

	if version {
		fmt.Printf(httpfs.FullVersion())
		os.Exit(0)
	}

	if !debug {
		log.SetOutput(ioutil.Discard)
	}

	http.Handle("/", webapi.FileServer(root, readonly))

	var handler http.Handler

	if debug {
		handler = Log(http.DefaultServeMux)
	} else {
		handler = http.DefaultServeMux
	}

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
