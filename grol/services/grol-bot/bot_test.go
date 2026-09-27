package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatRejectsTools(t *testing.T) {
	bot := NewBot("http://127.0.0.1:9")
	rr := httptest.NewRecorder()
	body := `{"messages":[{"role":"user","content":"hi"}],"tools":[{"name":"web_search"}]}`
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(body)))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestChatUsesSessionAndIdentity(t *testing.T) {
	var got map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": "ack"})
	}))
	defer upstream.Close()

	bot := NewBot(upstream.URL)
	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"status please"}]}`,
	)))
	if rr.Code != 200 {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	msgs, _ := got["messages"].([]any)
	if len(msgs) < 2 {
		t.Fatalf("expected identity + user, got %#v", got["messages"])
	}
	first, _ := msgs[0].(map[string]any)
	if first["role"] != "system" || !strings.Contains(first["content"].(string), "Grok Bot") {
		t.Fatalf("missing identity: %#v", first)
	}
	var reply map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &reply)
	if reply["session_id"] == nil || reply["session_id"] == "" {
		t.Fatalf("missing session id: %v", reply)
	}
	if bot.store.SessionCount() != 1 {
		t.Fatalf("sessions %d", bot.store.SessionCount())
	}
}

func TestHealthReportsBotNotGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ok":true,"service":"grol-ai-gateway","provisioned":true}`)
	}))
	defer upstream.Close()
	bot := NewBot(upstream.URL)
	rr := httptest.NewRecorder()
	bot.health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["service"] != "grol-bot" {
		t.Fatalf("health %v", body)
	}
}
