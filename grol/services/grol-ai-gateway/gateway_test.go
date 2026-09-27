package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthUnprovisioned(t *testing.T) {
	gw := NewGateway("", "/no/such/key")
	rr := httptest.NewRecorder()
	gw.health(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != 200 {
		t.Fatalf("status %d", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["provisioned"] != false || body["tools"] != false {
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
