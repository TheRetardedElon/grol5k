package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const consoleMaxBody = 64 * 1024

func main() {
	addr := flag.String("addr", "0.0.0.0:8790", "listen address")
	gw := flag.String("gateway", "http://127.0.0.1:8789", "grol-ai-gateway base URL")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
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
	w.Header().Set("Cache-Control", "no-store")
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
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&health); err == nil {
			health["reachable"] = true
			out["gateway"] = health
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

func proxyChat(w http.ResponseWriter, r *http.Request, gw string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, consoleMaxBody+1))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if len(body) > consoleMaxBody {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		strings.TrimRight(gw, "/")+"/v1/chat",
		strings.NewReader(string(body)),
	)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")

	client := &http.Client{Timeout: 30 * time.Minute}
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

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)

	if strings.HasPrefix(contentType, "text/event-stream") {
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				if _, writeErr := w.Write(buf[:n]); writeErr != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if readErr != nil {
				return
			}
		}
	}

	_, _ = io.Copy(w, resp.Body)
}
