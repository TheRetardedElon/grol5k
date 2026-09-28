package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func haobsProven(entity, renamed string) *httptest.Server {
	if renamed == "" {
		renamed = entity
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok", "registry_status": "ok",
			"devices": []map[string]any{{
				"entity_id": renamed, "domain": "light", "state": "off",
				"registry_id": "reg-kitchen", "platform": "hue", "identity_proven": true,
			}},
		})
	}))
}

func operatorGrant(t *testing.T, url string, body map[string]string) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url+"/v1/grants", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", "secret-op")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func TestNoGrantIsNotGranted(t *testing.T) {
	hs := haobsProven("light.kitchen", "")
	defer hs.Close()
	b := NewBroker(hs.URL)
	srv := httptest.NewServer(http.HandlerFunc(b.propose))
	defer srv.Close()
	_, out := postJSON(t, srv.URL, map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p := out["proposal"].(map[string]any)
	if p["state"] != "not_granted" || p["error"] != "not_granted" {
		t.Fatalf("%#v", p)
	}
}

func TestProvenGrantPendingConfirmation(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := haobsProven("light.kitchen", "")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	mux.HandleFunc("POST /v1/proposals/{id}/confirm", b.confirm)
	mux.HandleFunc("POST /v1/proposals/{id}/apply", b.apply)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	gout := operatorGrant(t, srv.URL, map[string]string{
		"entity_id": "light.kitchen", "service": "light.turn_on",
		"registry_id": "reg-kitchen", "platform": "hue",
	})
	if gout["grant"].(map[string]any)["authorizing"] != true {
		t.Fatalf("%#v", gout)
	}
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p := out["proposal"].(map[string]any)
	if p["state"] != "pending_confirmation" || p["decision"] != "confirmation_required" {
		t.Fatalf("%#v", p)
	}
	if p["apply_enabled"] != false || p["mutation_capable"] != false {
		t.Fatalf("%#v", p)
	}
	digest, _ := p["confirm_digest"].(string)
	if digest == "" {
		t.Fatal("missing digest")
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/proposals/"+p["id"].(string)+"/confirm", bytes.NewReader([]byte(`{"digest":"`+digest+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", "secret-op")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cout map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&cout)
	cp := cout["proposal"].(map[string]any)
	if cp["state"] != "confirmed" || cout["apply_enabled"] != false {
		t.Fatalf("%#v", cout)
	}
	rec := httptest.NewRecorder()
	b.apply(rec, httptest.NewRequest(http.MethodPost, "/v1/proposals/"+p["id"].(string)+"/apply", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("apply %d", rec.Code)
	}
}

func TestGrantFollowsRegistryRename(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := haobsProven("light.kitchen", "light.kitchen_2")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operatorGrant(t, srv.URL, map[string]string{
		"entity_id": "light.kitchen", "service": "light.turn_on",
		"registry_id": "reg-kitchen", "platform": "hue",
	})
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen_2"})
	p := out["proposal"].(map[string]any)
	if p["state"] != "pending_confirmation" {
		t.Fatalf("rename should follow registry_id %#v", p)
	}
}

func TestGrantIdentityDrift(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok",
			"devices": []map[string]any{{
				"entity_id": "light.kitchen", "domain": "light",
				"registry_id": "reg-kitchen", "platform": "mqtt", "identity_proven": true,
			}},
		})
	}))
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operatorGrant(t, srv.URL, map[string]string{
		"entity_id": "light.kitchen", "service": "light.turn_on",
		"registry_id": "reg-kitchen", "platform": "hue",
	})
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	if out["proposal"].(map[string]any)["error"] != "grant_identity_drift" {
		t.Fatalf("%#v", out)
	}
}
