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
	addr := flag.String("addr", "127.0.0.1:8790", "listen address (loopback by default; explicitly bind LAN only for development)")
	upstream := flag.String("bot", "http://127.0.0.1:8788", "grol-bot base URL")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, _ *http.Request) {
		status(w, *upstream)
	})
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		proxyChat(w, r, *upstream)
	})
	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		proxyRead(w, r, *upstream, "/v1/sessions")
	})
	mux.HandleFunc("GET /api/activity", func(w http.ResponseWriter, r *http.Request) {
		proxyRead(w, r, *upstream, "/v1/activity")
	})
	mux.HandleFunc("GET /api/system", func(w http.ResponseWriter, r *http.Request) {
		proxyRead(w, r, *upstream, "/v1/system")
	})
	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
		proxyRead(w, r, *upstream, "/v1/devices")
	})

	log.Printf("grol-console listen %s bot %s", *addr, *upstream)
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

func status(w http.ResponseWriter, upstream string) {
	out := map[string]any{
		"ok":      true,
		"product": "GROL5000",
		"service": "grol-console",
		"bot":      map[string]any{"reachable": false},
		"gateway":  map[string]any{"reachable": false},
		"observer":    map[string]any{"reachable": false},
		"ha_observer": map[string]any{"reachable": false},
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(strings.TrimRight(upstream, "/") + "/health")
	if err == nil {
		defer resp.Body.Close()
		var health map[string]any
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&health); err == nil {
			health["reachable"] = true
			out["bot"] = health
			if nested, ok := health["gateway"].(map[string]any); ok {
				out["gateway"] = nested
			}
			if nested, ok := health["observer"].(map[string]any); ok {
				out["observer"] = nested
			}
			if nested, ok := health["ha_observer"].(map[string]any); ok {
				out["ha_observer"] = nested
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

func proxyRead(w http.ResponseWriter, r *http.Request, upstream, path string) {
	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		strings.TrimRight(upstream, "/")+path,
		nil,
	)
	if err != nil {
		http.Error(w, "bad bot", http.StatusBadGateway)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    false,
			"error": "bot_unreachable",
		})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 2*1024*1024))
}

func proxyChat(w http.ResponseWriter, r *http.Request, upstream string) {
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
		strings.TrimRight(upstream, "/")+"/v1/chat",
		strings.NewReader(string(body)),
	)
	if err != nil {
		http.Error(w, "bad bot", http.StatusBadGateway)
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
			"text":     "grol-bot is not running. Start it on 127.0.0.1:8788.",
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
