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

func TestReadFailuresExitNonZero(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	for _, args := range [][]string{
		{"devices", "list"}, {"system", "status"}, {"activity", "recent"}, {"device", "get", "weather.forecast_home"},
	} {
		out, code := run(cli, args)
		if code != readFailureExit || out["ok"] != false {
			t.Fatalf("%v code=%d %#v", args, code, out)
		}
	}
}

func TestStatusSeparatesProcessFromLiveHouse(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/v1/snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "status": "ha_unreachable"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer hs.Close()
	cli := NewClient(hs.URL, "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"status"})
	if code != 0 || out["haobs"] != true || out["house_reachable"] != false || out["status"] != "degraded" {
		t.Fatalf("%#v", out)
	}
}

func TestSystemPropagatesDegradedSnapshot(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "status": "degraded", "errors": map[string]any{"network": "unreachable"},
		})
	}))
	defer hs.Close()
	cli := NewClient("http://127.0.0.1:9", hs.URL, "http://127.0.0.1:9")
	out, code := run(cli, []string{"system", "status"})
	if code != readFailureExit {
		t.Fatalf("expected failed read exit, got %d %#v", code, out)
	}
	if out["ok"] != false || out["status"] != "degraded" {
		t.Fatalf("degraded snapshot must propagate: %#v", out)
	}
}

func TestDevicesJSONFromHaobs(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/snapshot" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "mutation_capable": false, "untrusted": true, "count": 1,
			"devices": []map[string]any{{
				"entity_id": "weather.forecast_home", "domain": "weather", "state": "partlycloudy", "name": "Forecast Home",
			}},
		})
	}))
	defer hs.Close()
	cli := NewClient(hs.URL, "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"devices", "list"})
	if code != 0 || out["ok"] != true {
		t.Fatalf("devices list failed: %#v", out)
	}
	got, code := run(cli, []string{"device", "get", "weather.forecast_home"})
	if code != 0 || got["ok"] != true {
		t.Fatalf("device get failed: code=%d %#v", code, got)
	}
}

func TestProposeHitsBroker(t *testing.T) {
	brk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"proposal": map[string]any{
				"id": "abc", "service": "light.turn_on", "entity_id": "light.kitchen",
				"decision": "denied", "error": "unknown_entity",
			},
		})
	}))
	defer brk.Close()
	cli := NewClientWithBroker("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9", brk.URL)
	out, code := run(cli, []string{"propose", "light.turn_on", "light.kitchen"})
	if code != 0 || out["proposal"].(map[string]any)["error"] != "unknown_entity" {
		t.Fatalf("%d %#v", code, out)
	}
}

func TestUnknownVerb(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"shell"})
	if code != 2 || out["error"] != "unknown_verb" {
		t.Fatal(code, out["error"])
	}
}
