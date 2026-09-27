package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBotGrantDoesNotSendOperatorHeaders(t *testing.T) {
	actor, token := "", ""
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/grants", func(w http.ResponseWriter, r *http.Request) {
		actor = r.Header.Get("X-GROL-Actor")
		token = r.Header.Get("X-GROL-Operator-Token")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "operator_required"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli := NewClientWithBroker("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9", srv.URL)
	out, code := run(cli, []string{"grant", "add", "light.kitchen", "light.turn_on"})
	if actor == "operator" || token != "" {
		t.Fatalf("bot leaked operator headers actor=%q token=%q", actor, token)
	}
	if code == 0 || out["error"] != "operator_required" {
		t.Fatalf("%d %#v", code, out)
	}
}

func TestOperatorGrantSendsHeaders(t *testing.T) {
	actor, token := "", ""
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/grants", func(w http.ResponseWriter, r *http.Request) {
		actor = r.Header.Get("X-GROL-Actor")
		token = r.Header.Get("X-GROL-Operator-Token")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "grant": map[string]any{"authorizing": false}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli := NewClientWithBroker("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9", srv.URL)
	cli.OperatorToken = "secret-op"
	out, code := run(cli, []string{"grant", "add", "light.kitchen", "light.turn_on"})
	if code != 0 || out["ok"] != true {
		t.Fatalf("%d %#v", code, out)
	}
	if actor != "operator" || token != "secret-op" {
		t.Fatalf("headers actor=%q token=%q", actor, token)
	}
}

func TestOperatorConfirmSendsDigest(t *testing.T) {
	gotDigest := ""
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/proposals/abc", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"proposal": map[string]any{"id": "abc", "confirm_digest": "deadbeef", "state": "pending_confirmation"},
		})
	})
	mux.HandleFunc("/v1/proposals/abc/confirm", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotDigest, _ = body["digest"].(string)
		if r.Header.Get("X-GROL-Actor") != "operator" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "operator_required"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "proposal": map[string]any{"state": "confirmed", "apply_enabled": false}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli := NewClientWithBroker("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9", srv.URL)
	cli.OperatorToken = "secret-op"
	out, code := run(cli, []string{"proposal", "confirm", "abc"})
	if code != 0 || out["ok"] != true {
		t.Fatalf("%d %#v", code, out)
	}
	if gotDigest != "deadbeef" {
		t.Fatalf("digest %q", gotDigest)
	}
}
