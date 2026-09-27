package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHAObserverContextIsUntrustedAndTokenless(t *testing.T) {
	haobs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/snapshot" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "mutation_capable": false, "untrusted": true,
			"devices": []map[string]any{{"entity_id": "light.kitchen", "state": "on", "name": "Kitchen"}},
		})
	}))
	defer haobs.Close()

	var got map[string]any
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "text": "kitchen is on"})
	}))
	defer gateway.Close()

	bot := NewBotWithObservers(gateway.URL, "", haobs.URL)
	rr := httptest.NewRecorder()
	bot.chat(rr, httptest.NewRequest(http.MethodPost, "/v1/chat", strings.NewReader(
		`{"messages":[{"role":"user","content":"kitchen lights?"}]}`,
	)))

	msgs, _ := got["messages"].([]any)
	found := false
	for _, raw := range msgs {
		msg, _ := raw.(map[string]any)
		text, _ := msg["content"].(string)
		if strings.Contains(text, "grol-haobs") || strings.Contains(text, "light.kitchen") {
			found = true
			if !strings.Contains(text, "untrusted data") {
				t.Fatalf("missing untrusted marker: %q", text)
			}
			if strings.Contains(text, "Bearer") || strings.Contains(strings.ToLower(text), "token") {
				t.Fatalf("token leaked into model context: %q", text)
			}
		}
	}
	if !found {
		t.Fatalf("HA snapshot not injected: %#v", got["messages"])
	}
}
