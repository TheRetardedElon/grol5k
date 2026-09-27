package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusReportsGatewayHealth(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"service":"grol-ai-gateway","provisioned":true,"model":"test-model"}`)
	}))
	defer gateway.Close()

	rr := httptest.NewRecorder()
	status(rr, gateway.URL)

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	gw, _ := body["gateway"].(map[string]any)
	if gw["reachable"] != true || gw["provisioned"] != true || gw["model"] != "test-model" {
		t.Fatalf("unexpected status: %#v", body)
	}
}

func TestProxyChatStreamsSSE(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" {
			t.Fatalf("path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: delta\ndata: {\"text\":\"hello\"}\n\n")
		_, _ = io.WriteString(w, "event: done\ndata: {\"ok\":true}\n\n")
	}))
	defer gateway.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chat",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`),
	)
	proxyChat(rr, req, gateway.URL)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content type %q", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Body.String(), "event: delta") || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("unexpected stream: %s", rr.Body.String())
	}
}

func TestProxyChatDegradesWhenGatewayDown(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chat",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`),
	)
	proxyChat(rr, req, "http://127.0.0.1:1")

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["degraded"] != true {
		t.Fatalf("unexpected body: %#v", body)
	}
}
