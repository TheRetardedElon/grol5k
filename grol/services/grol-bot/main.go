package main

import (
	"flag"
	"log"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address")
	gw := flag.String("gateway", "http://127.0.0.1:8789", "grol-ai-gateway base URL")
	observer := flag.String("observer", "http://127.0.0.1:8787", "grol-healthd base URL")
	flag.Parse()
	bot := NewBotWithObserver(*gw, *observer)
	log.Printf("grol-bot listen %s gateway %s observer %s", *addr, *gw, *observer)
	if err := bot.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
