package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func haobsOK(entity string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok",
			"devices": []map[string]any{{
				"entity_id": entity, "domain": "light", "state": "off",
			}},
		})
	}))
}

func postJSON(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestUnknownStillDenied(t *testing.T) {
	hs := haobsOK("light.other")
	defer hs.Close()
	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()
	_, out := postJSON(t, srv.URL, map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p := out["proposal"].(map[string]any)
	if p["state"] != "denied" || p["error"] != "unknown_entity" {
		t.Fatalf("%#v", p)
	}
}
