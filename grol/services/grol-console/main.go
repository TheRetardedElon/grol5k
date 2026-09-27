package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:8790", "listen address")
	gw := flag.String("gateway", "http://127.0.0.1:8789", "grol-ai-gateway base URL")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		status(w, *gw)
	})
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		proxyChat(w, r, *gw)
	})
	log.Printf("grol-console listen %s gateway %s", *addr, *gw)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func status(w http.ResponseWriter, gw string) {
	out := map[string]any{
		"ok":      true,
		"product": "GROL5000",
		"service": "grol-console",
		"gateway": map[string]any{"reachable": false},
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(gw, "/") + "/health")
	if err == nil {
		defer resp.Body.Close()
		var health map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&health)
		out["gateway"] = health
		health["reachable"] = true
		out["gateway"] = health
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func proxyChat(w http.ResponseWriter, r *http.Request, gw string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(gw, "/")+"/v1/chat", strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"degraded": true,
			"text":     "grol-ai-gateway is not running. Start it on 127.0.0.1:8789.",
		})
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func init() {
	_ = os.DevNull
}
