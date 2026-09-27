package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMutationVerbsRefused(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	for _, args := range [][]string{
		{"propose", "light.turn_on", "kitchen"},
		{"raw-call"},
		{"turn_on"},
	} {
		out, code := run(cli, args)
		if code != 3 {
			t.Fatalf("%v code=%d", args, code)
		}
		if out["error"] != "mutation_disabled" {
			t.Fatalf("%v error=%v", args, out["error"])
		}
		if out["mutation_capable"] != false {
			t.Fatalf("mutation must stay false")
		}
	}
}

func TestUnknownVerb(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"shell"})
	if code != 2 || out["error"] != "unknown_verb" {
		t.Fatalf("got code=%d err=%v", code, out["error"])
	}
}

func TestReadFailuresExitNonZero(t *testing.T) {
	cli := NewClient("http://127.0.0.1:9", "http://127.0.0.1:9", "http://127.0.0.1:9")
	for _, args := range [][]string{
		{"devices", "list"},
		{"system", "status"},
		{"activity", "recent"},
		{"device", "get", "weather.forecast_home"},
	} {
		out, code := run(cli, args)
		if code != readFailureExit {
			t.Fatalf("%v code=%d out=%#v", args, code, out)
		}
		if out["ok"] != false {
			t.Fatalf("%v should be ok=false: %#v", args, out)
		}
	}
}

func TestDevicesJSONFromHaobs(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/snapshot" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":               true,
			"mutation_capable": false,
			"untrusted":        true,
			"count":            1,
			"devices": []map[string]any{{
				"entity_id": "weather.forecast_home",
				"domain":    "weather",
				"state":     "partlycloudy",
				"name":      "Forecast Home",
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

func TestStatusSeparatesProcessFromLiveHouse(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		case "/v1/snapshot":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":     false,
				"status": "ha_unreachable",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer hs.Close()

	cli := NewClient(hs.URL, "http://127.0.0.1:9", "http://127.0.0.1:9")
	out, code := run(cli, []string{"status"})
	if code != 0 || out["ok"] != true {
		t.Fatalf("status command itself should succeed: code=%d %#v", code, out)
	}
	if out["haobs"] != true {
		t.Fatalf("haobs process should be reachable: %#v", out)
	}
	if out["house_reachable"] != false {
		t.Fatalf("house must be reported unreachable: %#v", out)
	}
	if out["status"] != "degraded" {
		t.Fatalf("expected degraded stack: %#v", out)
	}
}

func TestSystemPropagatesDegradedSnapshot(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     false,
			"status": "degraded",
			"errors": map[string]any{"network": "unreachable"},
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
