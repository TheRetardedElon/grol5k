package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	maxBody       = 64 * 1024
	defaultXAIURL = "https://api.x.ai"
	defaultModel  = "grok-4.7"
)

// Gateway is the only process allowed to hold xAI credentials.
// It never talks to Docker, hostd, Home Assistant, or the future action broker.
type Gateway struct {
	key     string
	baseURL string
	model   string
	client  *http.Client
}

func NewGateway(envKey, keyFile string) *Gateway {
	key := strings.TrimSpace(envKey)
	if key == "" {
		if b, err := os.ReadFile(keyFile); err == nil {
			key = strings.TrimSpace(string(b))
		}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("XAI_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultXAIURL
	}
	model := strings.TrimSpace(os.Getenv("XAI_MODEL"))
	if model == "" {
		model = defaultModel
	}

	return &Gateway{
		key:     key,
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			// Reasoning models can take a while before the first token. The
			// browser/console owns cancellation; this is only a hard ceiling.
			Timeout: 30 * time.Minute,
		},
	}
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
		"model":       g.model,
		"tools":       false,
		"streaming":   true,
	})
}

type chatRequest struct {
	SessionID string           `json:"session_id"`
	Messages  []chatMessage    `json:"messages"`
	Tools     []map[string]any `json:"tools"`
	Stream    bool             `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type xaiResponseRequest struct {
	Model  string        `json:"model"`
	Input  []chatMessage `json:"input"`
	Stream bool          `json:"stream"`
	Store  bool          `json:"store"`
}

type xaiResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

type xaiStreamEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
	Error any    `json:"error,omitempty"`
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
	if err := validateMessages(req.Messages); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "invalid_messages",
		})
		return
	}

	if !g.Provisioned() {
		last := lastUserMessage(req.Messages)
		if req.Stream {
			beginSSE(w)
			writeSSE(w, "delta", map[string]any{
				"text": "Grok is not provisioned on this machine. Set XAI_API_KEY or /etc/grol/xai.key.",
			})
			writeSSE(w, "done", map[string]any{
				"ok":       true,
				"degraded": true,
				"provider": "none",
				"echo":     last,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"degraded": true,
			"provider": "none",
			"text":     "Grok is not provisioned on this machine. Set XAI_API_KEY or /etc/grol/xai.key.",
			"echo":     last,
		})
		return
	}

	if req.Stream {
		g.streamChat(w, r, req)
		return
	}
	g.completeChat(w, r, req)
}

func validateMessages(messages []chatMessage) error {
	if len(messages) == 0 || len(messages) > 64 {
		return errors.New("message count")
	}
	for _, m := range messages {
		switch m.Role {
		case "system", "user", "assistant":
		default:
			return errors.New("role")
		}
		if strings.TrimSpace(m.Content) == "" {
			return errors.New("empty content")
		}
	}
	return nil
}

func lastUserMessage(messages []chatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func (g *Gateway) newProviderRequest(r *http.Request, messages []chatMessage, stream bool) (*http.Request, error) {
	payload, err := json.Marshal(xaiResponseRequest{
		Model:  g.model,
		Input:  messages,
		Stream: stream,
		// GROL does not ask xAI to persist household/operator conversations.
		Store: false,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodPost,
		g.baseURL+"/v1/responses",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+g.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	return req, nil
}

func (g *Gateway) completeChat(w http.ResponseWriter, r *http.Request, req chatRequest) {
	upstream, err := g.newProviderRequest(r, req.Messages, false)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "provider_request"})
		return
	}
	upstream.Header.Set("Accept", "application/json")

	resp, err := g.client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "provider_unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":              false,
			"error":           "provider_error",
			"provider_status": resp.StatusCode,
		})
		return
	}

	var out xaiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&out); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "provider_decode"})
		return
	}

	var text strings.Builder
	for _, item := range out.Output {
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text.WriteString(part.Text)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"degraded": false,
		"provider": "xai",
		"model":    g.model,
		"text":     text.String(),
	})
}

func (g *Gateway) streamChat(w http.ResponseWriter, r *http.Request, req chatRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "streaming_unsupported"})
		return
	}

	upstream, err := g.newProviderRequest(r, req.Messages, true)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "provider_request"})
		return
	}

	resp, err := g.client.Do(upstream)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": "provider_unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":              false,
			"error":           "provider_error",
			"provider_status": resp.StatusCode,
		})
		return
	}

	beginSSE(w)
	flusher.Flush()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 32*1024), 1024*1024)
	done := false

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var event xaiStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "response.output_text.delta", "response.text.delta":
			if event.Delta != "" {
				writeSSE(w, "delta", map[string]any{"text": event.Delta})
				flusher.Flush()
			}
		case "response.completed":
			writeSSE(w, "done", map[string]any{
				"ok":       true,
				"degraded": false,
				"provider": "xai",
				"model":    g.model,
			})
			flusher.Flush()
			done = true
		case "response.failed", "error":
			writeSSE(w, "error", map[string]any{
				"ok":    false,
				"error": "provider_stream_error",
			})
			flusher.Flush()
			return
		}
	}

	if err := scanner.Err(); err != nil {
		writeSSE(w, "error", map[string]any{"ok": false, "error": "provider_stream_read"})
		flusher.Flush()
		return
	}
	if !done {
		writeSSE(w, "done", map[string]any{
			"ok":       true,
			"degraded": false,
			"provider": "xai",
			"model":    g.model,
		})
		flusher.Flush()
	}
}

func beginSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSE(w io.Writer, event string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	_, _ = io.WriteString(w, "event: "+event+"\n")
	_, _ = io.WriteString(w, "data: "+string(b)+"\n\n")
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
