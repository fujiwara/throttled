package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/fujiwara/throttled"
	isatty "github.com/mattn/go-isatty"
)

func main() {
	var port, size int
	var accessLog bool
	flag.IntVar(&port, "port", 0, "Listen port")
	flag.IntVar(&size, "size", 100000, "Cache size")
	flag.BoolVar(&accessLog, "accesslog", false, "Enable output access log")
	flag.Parse()
	if port == 0 {
		log.Println("-port required")
		os.Exit(1)
	}
	addr := fmt.Sprintf(":%d", port)
	log.Printf("throttled starting up on %s", addr)

	var h http.Handler
	if accessLog {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			h = throttled.Handler(os.Stdout)
		} else {
			h = throttled.Handler(bufio.NewWriter(os.Stdout))
		}
	} else {
		h = throttled.Handler(nil)
	}
	throttled.Setup(size)
	log.Fatal(http.ListenAndServe(addr, h))
}
