package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"grael/demo/server"
)

func main() {
	fs := flag.NewFlagSet("grael-demo", flag.ExitOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	addr := fs.String("addr", ":4000", "HTTP listen address")
	fs.Parse(os.Args[1:])

	srv := server.New(*dataDir)
	defer srv.Close()

	fmt.Printf("grael demo listening on http://localhost%s\n", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
