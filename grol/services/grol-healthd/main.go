package main

import (
	"flag"
	"log"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address (loopback only)")
	hostSocket := flag.String("host-socket", "/run/grol/hostapi.sock", "grol-hostd unix socket")
	flag.Parse()

	h := NewHealthd(*hostSocket)
	log.Printf("grol-healthd listen %s hostd %s", *addr, *hostSocket)
	if err := h.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
