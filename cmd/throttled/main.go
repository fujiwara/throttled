package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/fujiwara/throttled"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 0, "Listen port")
	flag.Parse()
	if port == 0 {
		log.Println("-port required")
		os.Exit(1)
	}
	addr := fmt.Sprintf(":%d", port)
	log.Printf("throttled starting up on %s", addr)
	log.Fatal(http.ListenAndServe(addr, throttled.Handler(os.Stdout)))
}
