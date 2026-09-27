package main

import (
	"flag"
	"log"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8785", "listen address")
	haobs := flag.String("ha-observer", "http://127.0.0.1:8786", "grol-haobs base URL")
	flag.Parse()
	b := NewBroker(*haobs)
	log.Printf("grol-action-broker listen %s ha-observer %s apply=disabled", *addr, *haobs)
	if err := b.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
