package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8789", "listen address (loopback by default)")
	keyFile := flag.String("key-file", "/etc/grol/xai.key", "xAI API key file")
	flag.Parse()
	gw := NewGateway(os.Getenv("XAI_API_KEY"), *keyFile)
	log.Printf("grol-ai-gateway listen %s provisioned=%v", *addr, gw.Provisioned())
	if err := gw.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}
