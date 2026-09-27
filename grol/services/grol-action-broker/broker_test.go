package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func postPropose(t *testing.T, url, service, entity string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"service": service, "entity_id": entity})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestProposeUnknownAndIneligible(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok",
			"devices": []map[string]any{{
				"entity_id": "weather.forecast_home", "domain": "weather", "state": "rainy",
			}},
		})
	}))
	defer hs.Close()
	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()

	inel := postPropose(t, srv.URL, "script.turn_on", "script.foo")
	if inel["proposal"].(map[string]any)["error"] != "ineligible_service" {
		t.Fatalf("%#v", inel)
	}
	unk := postPropose(t, srv.URL, "light.turn_on", "light.kitchen")
	if unk["proposal"].(map[string]any)["error"] != "unknown_entity" {
		t.Fatalf("%#v", unk)
	}
	mis := postPropose(t, srv.URL, "light.turn_on", "weather.forecast_home")
	if mis["proposal"].(map[string]any)["error"] != "entity_service_mismatch" {
		t.Fatalf("%#v", mis)
	}
}

func TestUnhealthyHaobsIsUnavailableNotUnknown(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": false, "status": "ha_unreachable", "devices": []any{},
		})
	}))
	defer hs.Close()
	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()
	out := postPropose(t, srv.URL, "light.turn_on", "light.kitchen")
	if out["proposal"].(map[string]any)["error"] != "haobs_unavailable" {
		t.Fatalf("want haobs_unavailable got %#v", out)
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

func TestListenRejectsNonLoopback(t *testing.T) {
	if err := ListenAddr("0.0.0.0:8785"); err == nil {
		t.Fatal("LAN bind must fail closed")
	}
	if err := ListenAddr("127.0.0.1:8785"); err != nil {
		t.Fatal(err)
	}
}
