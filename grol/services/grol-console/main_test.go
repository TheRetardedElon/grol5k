package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusReportsBotAndNestedGatewayHealth(t *testing.T) {
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"ok":true,
			"service":"grol-bot",
			"sessions":2,
			"gateway":{
				"ok":true,
				"service":"grol-ai-gateway",
				"provisioned":true,
				"model":"test-model",
				"reachable":true
			},
			"observer":{
				"ok":true,
				"service":"grol-healthd",
				"reachable":true,
				"hostd":{"reachable":true}
			},
			"ha_observer":{
				"ok":true,
				"service":"grol-haobs",
				"provisioned":true,
				"reachable":true,
				"mutation_capable":false
			}
		}`)
	}))
	defer bot.Close()

	rr := httptest.NewRecorder()
	status(rr, bot.URL)

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	botHealth, _ := body["bot"].(map[string]any)
	if botHealth["reachable"] != true || botHealth["service"] != "grol-bot" {
		t.Fatalf("unexpected bot status: %#v", body)
	}
	gw, _ := body["gateway"].(map[string]any)
	if gw["reachable"] != true || gw["provisioned"] != true || gw["model"] != "test-model" {
		t.Fatalf("unexpected gateway status: %#v", body)
	}
	observer, _ := body["observer"].(map[string]any)
	if observer["reachable"] != true || observer["service"] != "grol-healthd" {
		t.Fatalf("unexpected observer status: %#v", body)
	}
	haObserver, _ := body["ha_observer"].(map[string]any)
	if haObserver["reachable"] != true || haObserver["service"] != "grol-haobs" || haObserver["provisioned"] != true {
		t.Fatalf("unexpected HA observer status: %#v", body)
	}
}

func TestProxyChatStreamsSSE(t *testing.T) {
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" {
			t.Fatalf("path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: session\ndata: {\"session_id\":\"abc\"}\n\n")
		_, _ = io.WriteString(w, "event: delta\ndata: {\"text\":\"hello\"}\n\n")
		_, _ = io.WriteString(w, "event: done\ndata: {\"ok\":true}\n\n")
	}))
	defer bot.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/chat",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`),
	)
	proxyChat(rr, req, bot.URL)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content type %q", rr.Header().Get("Content-Type"))
	}
	out := rr.Body.String()
	if !strings.Contains(out, "event: session") ||
		!strings.Contains(out, "event: delta") ||
		!strings.Contains(out, "event: done") {
		t.Fatalf("unexpected stream: %s", out)
	}
}

func TestProxyReadSessionsAndActivity(t *testing.T) {
	bot := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sessions":
			_, _ = io.WriteString(w, `{"ok":true,"sessions":[{"id":"abc","turns":2}]}`)
		case "/v1/activity":
			_, _ = io.WriteString(w, `{"ok":true,"activity":[{"session_id":"abc","role":"user","preview":"hi"}]}`)
		case "/v1/system":
			_, _ = io.WriteString(w, `{"ok":true,"snapshot":{"status":"ok","system":{"grol_version":"18.4.dev"}}}`)
		case "/v1/devices":
			_, _ = io.WriteString(w, `{"ok":true,"snapshot":{"status":"ok","mutation_capable":false,"untrusted":true,"count":1,"devices":[{"entity_id":"weather.forecast_home","domain":"weather","state":"partlycloudy","name":"Forecast home"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer bot.Close()

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/v1/sessions", "\"sessions\""},
		{"/v1/activity", "\"activity\""},
		{"/v1/system", "\"snapshot\""},
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/read", nil)
		proxyRead(rr, req, bot.URL, tc.path)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status %d", tc.path, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), tc.want) {
			t.Fatalf("%s unexpected body %s", tc.path, rr.Body.String())
		}
	}
}

func TestProxyChatDegradesWhenBotDown(t *testing.T) {
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
