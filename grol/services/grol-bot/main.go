package main

import (
	"flag"
	"log"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address")
	gw := flag.String("gateway", "http://127.0.0.1:8789", "grol-ai-gateway base URL")
	flag.Parse()
	bot := NewBot(*gw)
	log.Printf("grol-bot listen %s gateway %s", *addr, *gw)
	if err := bot.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
