package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegrantAfterRenameThenRevokeDoesNotResurrect(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	name := "light.kitchen"
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok",
			"devices": []map[string]any{{
				"entity_id": name, "domain": "light",
				"registry_id": "reg-kitchen", "platform": "hue", "identity_proven": true,
			}},
		})
	}))
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("DELETE /v1/grants/{entity}/{service}", b.revokeGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	name = "light.kitchen_2"
	operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen_2", "service": "light.turn_on"})
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/v1/grants/light.kitchen_2/light.turn_on", nil)
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", "secret-op")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen_2"})
	if out["proposal"].(map[string]any)["error"] != "not_granted" {
		t.Fatalf("stale grant resurrected %#v", out)
	}
}

func TestRevokeFailsClosedWhenHaobsDown(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	b := NewBroker("http://127.0.0.1:9")
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /v1/grants/{entity}/{service}", b.revokeGrant)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/v1/grants/light.kitchen/light.turn_on", nil)
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", "secret-op")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusServiceUnavailable || out["error"] != "haobs_unavailable" {
		t.Fatalf("%d %#v", resp.StatusCode, out)
	}
}
