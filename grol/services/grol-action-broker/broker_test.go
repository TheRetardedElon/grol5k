package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProposeUnknownAndIneligible(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"devices": []map[string]any{{
				"entity_id": "weather.forecast_home",
				"domain":    "weather",
				"state":     "rainy",
			}},
		})
	}))
	defer hs.Close()

	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()

	post := func(service, entity string) map[string]any {
		body, _ := json.Marshal(map[string]string{"service": service, "entity_id": entity})
		resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out
	}

	inel := post("script.turn_on", "script.foo")
	p, _ := inel["proposal"].(map[string]any)
	if p["error"] != "ineligible_service" {
		t.Fatalf("script: %#v", inel)
	}

	unk := post("light.turn_on", "light.kitchen")
	p, _ = unk["proposal"].(map[string]any)
	if p["error"] != "unknown_entity" {
		t.Fatalf("kitchen: %#v", unk)
	}

	mismatch := post("light.turn_on", "weather.forecast_home")
	p, _ = mismatch["proposal"].(map[string]any)
	if p["error"] != "entity_service_mismatch" {
		t.Fatalf("weather as light: %#v", mismatch)
	}
}

func TestApplyDisabled(t *testing.T) {
	b := NewBroker("http://127.0.0.1:9")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/proposals/x/apply", nil)
	b.apply(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code %d", rec.Code)
	}
}
