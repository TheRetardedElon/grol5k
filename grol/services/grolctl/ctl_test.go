package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRawCallStillForbidden(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	for _, args := range [][]string{{"raw-call"}, {"turn_on"}, {"apply"}} {
		out, code := run(cli, args)
		if code != 3 || out["error"] != "mutation_disabled" {
			t.Fatalf("%v code=%d out=%v", args, code, out["error"])
		}
	}
}

func TestProposeUsage(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	_, code := run(cli, []string{"propose"})
	if code != 2 {
		t.Fatalf("code %d", code)
	}
}

func TestProposeHitsBroker(t *testing.T) {
	brk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/propose" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"proposal": map[string]any{
				"id": "abc", "service": "light.turn_on", "entity_id": "light.kitchen",
				"decision": "denied", "error": "unknown_entity",
				"mutation_capable": false,
			},
		})
	}))
	defer brk.Close()
	cli := NewClientWithBroker("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9", brk.URL)
	out, code := run(cli, []string{"propose", "light.turn_on", "light.kitchen"})
	if code != 0 || out["ok"] != true {
		t.Fatalf("%d %#v", code, out)
	}
	p := out["proposal"].(map[string]any)
	if p["error"] != "unknown_entity" {
		t.Fatalf("%#v", p)
	}
}

func TestUnknownVerb(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"shell"})
	if code != 2 || out["error"] != "unknown_verb" {
		t.Fatalf("got code=%d err=%v", code, out["error"])
	}
}
