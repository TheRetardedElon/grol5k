package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8786", "listen address")
	haURL := flag.String("ha-url", envDefault("HA_URL", "http://127.0.0.1:8123"), "Home Assistant base URL")
	tokenFile := flag.String("token-file", "/etc/grol/ha.token", "HA long-lived token file")
	flag.Parse()
	o := NewObserver(*haURL, os.Getenv("HA_TOKEN"), *tokenFile)
	log.Printf("grol-haobs listen %s ha %s provisioned=%v", *addr, *haURL, o.Provisioned())
	if err := o.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
