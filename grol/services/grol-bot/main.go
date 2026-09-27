package main

import (
	"flag"
	"log"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8788", "listen address")
	gw := flag.String("gateway", "http://127.0.0.1:8789", "grol-ai-gateway base URL")
	observer := flag.String("observer", "http://127.0.0.1:8787", "grol-healthd base URL")
	haobs := flag.String("ha-observer", "http://127.0.0.1:8786", "grol-haobs base URL")
	flag.Parse()
	bot := NewBotWithObservers(*gw, *observer, *haobs)
	log.Printf("grol-bot listen %s gateway %s host-observer %s ha-observer %s", *addr, *gw, *observer, *haobs)
	if err := bot.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
