package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

const maxBody = 64 * 1024

// Gateway is the only process allowed to hold xAI credentials.
// It never talks to Docker, hostd, or Home Assistant.
type Gateway struct {
	key string
}

func NewGateway(envKey, keyFile string) *Gateway {
	key := strings.TrimSpace(envKey)
	if key == "" {
		if b, err := os.ReadFile(keyFile); err == nil {
			key = strings.TrimSpace(string(b))
		}
	}
	return &Gateway{key: key}
}

func (g *Gateway) Provisioned() bool { return g.key != "" }

func (g *Gateway) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", g.health)
	mux.HandleFunc("POST /v1/chat", g.chat)
	return http.ListenAndServe(addr, mux)
}

func (g *Gateway) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"service":     "grol-ai-gateway",
		"provider":    "xai",
		"provisioned": g.Provisioned(),
		"tools":       false,
	})
}

type chatRequest struct {
	SessionID string         `json:"session_id"`
	Messages  []chatMessage  `json:"messages"`
	Tools     []map[string]any `json:"tools"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (g *Gateway) chat(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "bad_body"})
		return
	}
	if len(body) > maxBody {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"ok": false, "error": "too_large"})
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	if len(req.Tools) > 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"ok":    false,
			"error": "provider_tools_disabled",
		})
		return
	}
	last := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			last = req.Messages[i].Content
			break
		}
	}
	if !g.Provisioned() {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"degraded": true,
			"provider": "none",
			"text":     "Grok is not provisioned on this appliance. Set XAI_API_KEY or /etc/grol/xai.key.",
			"echo":     last,
		})
		return
	}
	// Live xAI streaming lands in a later M3A commit. Provisioned + no tools
	// is enough to prove the credential/tool boundary.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"degraded": false,
		"provider": "xai",
		"text":     "(gateway ready; live Grok stream not wired in this skeleton)",
		"echo":     last,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
