package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthUnprovisioned(t *testing.T) {
	t.Setenv("XAI_MODEL", "")
	gw := NewGateway("", "/no/such/key")
	rr := httptest.NewRecorder()
	gw.health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["provisioned"] != false || body["tools"] != false || body["streaming"] != true {
		t.Fatalf("unexpected health: %v", body)
	}
}

func TestChatRejectsTools(t *testing.T) {
	gw := NewGateway("secret", "")
	payload, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"tools":    []map[string]any{{"name": "web_search"}},
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewReader(payload))
	gw.chat(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestChatDegradedWithoutKey(t *testing.T) {
	gw := NewGateway("", "/no/such/key")
	payload, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "turn on lights"}},
	})
	rr := httptest.NewRecorder()
	gw.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewReader(payload)))
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["degraded"] != true || body["echo"] != "turn on lights" {
		t.Fatalf("unexpected chat: %v", body)
	}
}

func TestChatRejectsInvalidRole(t *testing.T) {
	gw := NewGateway("secret", "")
	payload, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "tool", "content": "not allowed"}},
	})
	rr := httptest.NewRecorder()
	gw.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewReader(payload)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestLiveCompletionUsesResponsesAPIWithoutToolsOrStorage(t *testing.T) {
	var gotAuth string
	var got map[string]any

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("provider path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"output":[{"type":"message","content":[{"type":"output_text","text":"hello from Grok"}]}]}`)
	}))
	defer provider.Close()

	gw := NewGateway("secret", "")
	gw.baseURL = provider.URL
	gw.client = provider.Client()
	gw.model = "test-model"

	payload, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hello"}},
	})
	rr := httptest.NewRecorder()
	gw.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewReader(payload)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("authorization %q", gotAuth)
	}
	if got["model"] != "test-model" || got["stream"] != false || got["store"] != false {
		t.Fatalf("unexpected provider payload: %#v", got)
	}
	if _, exists := got["tools"]; exists {
		t.Fatalf("provider tools must not be sent: %#v", got)
	}

	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["text"] != "hello from Grok" {
		t.Fatalf("unexpected response: %v", body)
	}
}

func TestLiveStreamNormalizesProviderSSE(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		if body["stream"] != true || body["store"] != false {
			t.Fatalf("unexpected provider payload: %#v", body)
		}
		if _, exists := body["tools"]; exists {
			t.Fatalf("provider tools must not be sent")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: response.output_text.delta\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello \"}\n\n")
		if f != nil {
			f.Flush()
		}
		_, _ = io.WriteString(w, "event: response.output_text.delta\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n")
		_, _ = io.WriteString(w, "event: response.completed\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\"}\n\n")
	}))
	defer provider.Close()

	gw := NewGateway("secret", "")
	gw.baseURL = provider.URL
	gw.client = provider.Client()
	gw.model = "test-model"

	payload, _ := json.Marshal(map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "hello"}},
		"stream":   true,
	})
	rr := httptest.NewRecorder()
	gw.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", bytes.NewReader(payload)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type %q", ct)
	}
	out := rr.Body.String()
	if !strings.Contains(out, `event: delta
data: {"text":"hello "}`) ||
		!strings.Contains(out, `data: {"text":"world"}`) ||
		!strings.Contains(out, `event: done`) {
		t.Fatalf("unexpected normalized stream:\n%s", out)
	}
}
