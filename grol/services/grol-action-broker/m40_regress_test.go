package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestM40IneligibleMismatchHaobs(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "ok", "devices": []any{}})
	}))
	defer hs.Close()
	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()
	_, inel := postJSON(t, srv.URL, map[string]string{"service": "script.turn_on", "entity_id": "script.foo"})
	if inel["proposal"].(map[string]any)["error"] != "ineligible_service" {
		t.Fatalf("%#v", inel)
	}
	_, mis := postJSON(t, srv.URL, map[string]string{"service": "light.turn_on", "entity_id": "switch.x"})
	if mis["proposal"].(map[string]any)["error"] != "entity_service_mismatch" {
		t.Fatalf("%#v", mis)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "status": "ha_unreachable"})
	}))
	defer bad.Close()
	b2 := NewBroker(bad.URL)
	srv2 := httptest.NewServer(http.HandlerFunc(b2.propose))
	defer srv2.Close()
	_, u := postJSON(t, srv2.URL, map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	if u["proposal"].(map[string]any)["error"] != "haobs_unavailable" {
		t.Fatalf("%#v", u)
	}
}

func TestListenRejectsNonLoopback(t *testing.T) {
	if err := ListenAddr("0.0.0.0:8785"); err == nil {
		t.Fatal("LAN bind must fail closed")
	}
}

func TestApplyDisabled(t *testing.T) {
	b := NewBroker("http://127.0.0.1:9")
	rec := httptest.NewRecorder()
	b.apply(rec, httptest.NewRequest(http.MethodPost, "/v1/proposals/x/apply", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code %d", rec.Code)
	}
}
