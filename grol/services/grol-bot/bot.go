package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxBody        = 64 * 1024
	identityPrompt = "You are Grok Bot, the resident operator of GROL5000 (Global Robotic Overlord Logic). You live on the household appliance. You do not have root, Docker, hostd, or Home Assistant mutation rights. Propose actions in plain language only. Never claim you already changed a device."
)

type Bot struct {
	gateway string
	store   *Store
	client  *http.Client
}

func NewBot(gateway string) *Bot {
	return &Bot{
		gateway: strings.TrimRight(gateway, "/"),
		store:   NewStore(),
		client:  &http.Client{Timeout: 30 * time.Minute},
	}
}

func (b *Bot) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", b.health)
	mux.HandleFunc("GET /v1/sessions", b.listSessions)
	mux.HandleFunc("GET /v1/activity", b.activity)
	mux.HandleFunc("POST /v1/chat", b.chat)
	return http.ListenAndServe(addr, mux)
}

func (b *Bot) health(w http.ResponseWriter, _ *http.Request) {
	gw := b.gatewayHealth()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"service":  "grol-bot",
		"identity": "grol-bot",
		"product":  "GROL5000",
		"sessions": b.store.SessionCount(),
		"gateway":  gw,
		"proposals": []any{},
	})
}

func (b *Bot) gatewayHealth() map[string]any {
	out := map[string]any{"reachable": false}
	resp, err := b.client.Get(b.gateway + "/health")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var health map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&health); err == nil {
		health["reachable"] = true
		return health
	}
	return out
}

func (b *Bot) listSessions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"sessions": b.store.Summaries(),
	})
}

func (b *Bot) activity(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"activity": b.store.Activity(),
	})
}

type chatRequest struct {
	SessionID string           `json:"session_id"`
	Messages  []Message        `json:"messages"`
	Tools     []map[string]any `json:"tools"`
	Stream    bool             `json:"stream"`
}

func (b *Bot) chat(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "bad_body"})
		return
	}
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	if len(req.Tools) > 0 {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "provider_tools_disabled"})
		return
	}

	sess := b.store.GetOrCreate(req.SessionID)
	user := lastUser(req.Messages)
	if user == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "missing_user_message"})
		return
	}
	b.store.Append(sess.ID, Message{Role: "user", Content: user})
	b.store.Note(sess.ID, "user", user)

	outgoing := []Message{{Role: "system", Content: identityPrompt}}
	outgoing = append(outgoing, sess.Messages...)

	payload, _ := json.Marshal(map[string]any{
		"session_id": sess.ID,
		"messages":   outgoing,
		"stream":     req.Stream,
	})
	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodPost, b.gateway+"/v1/chat", strings.NewReader(string(payload)))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "gateway_request"})
		return
	}
	upstream.Header.Set("Content-Type", "application/json")
	upstream.Header.Set("Accept", "text/event-stream, application/json")

	resp, err := b.client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"degraded":   true,
			"session_id": sess.ID,
			"text":       "grol-ai-gateway is not running.",
		})
		return
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
	b.proxySSE(w, resp, sess.ID)
		return
	}
	var reply map[string]any
	_ = json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&reply)
	if text, _ := reply["text"].(string); text != "" {
		b.store.Append(sess.ID, Message{Role: "assistant", Content: text})
		b.store.Note(sess.ID, "assistant", text)
	}
	reply["session_id"] = sess.ID
	reply["proposals"] = []any{}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_ = json.NewEncoder(w).Encode(reply)
}

func (b *Bot) proxySSE(w http.ResponseWriter, resp *http.Response, sessionID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	var assistant strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = w.Write(chunk)
			if flusher != nil {
				flusher.Flush()
			}
			collectDeltas(chunk, &assistant)
		}
		if err != nil {
			break
		}
	}
	if text := assistant.String(); text != "" {
		b.store.Append(sessionID, Message{Role: "assistant", Content: text})
		b.store.Note(sessionID, "assistant", text)
	}
}

func collectDeltas(chunk []byte, dst *strings.Builder) {
	for _, line := range strings.Split(string(chunk), "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &payload); err != nil {
			continue
		}
		if text, _ := payload["text"].(string); text != "" {
			dst.WriteString(text)
		}
	}
}

func lastUser(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i].Content
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
