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

func TestGrantAddResolvesLiveIdentity(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := haobsProven("light.kitchen", "")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	gout := operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	g := gout["grant"].(map[string]any)
	if g["authorizing"] != true || g["registry_id"] != "reg-kitchen" || g["platform"] != "hue" {
		t.Fatalf("%#v", gout)
	}
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	if out["proposal"].(map[string]any)["state"] != "pending_confirmation" {
		t.Fatalf("%#v", out)
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
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p := out["proposal"].(map[string]any)
	digest := p["confirm_digest"].(string)
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
	if cout["proposal"].(map[string]any)["state"] != "confirmed" || cout["apply_enabled"] != false {
		t.Fatalf("%#v", cout)
	}
}

func TestGrantFollowsRegistryRename(t *testing.T) {
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
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	name = "light.kitchen_2"
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen_2"})
	if out["proposal"].(map[string]any)["state"] != "pending_confirmation" {
		t.Fatalf("%#v", out)
	}
}

func TestGrantIdentityDrift(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	platform := "hue"
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "status": "ok",
			"devices": []map[string]any{{
				"entity_id": "light.kitchen", "domain": "light",
				"registry_id": "reg-kitchen", "platform": platform, "identity_proven": true,
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
	operatorGrant(t, srv.URL, map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	platform = "mqtt"
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	if out["proposal"].(map[string]any)["error"] != "grant_identity_drift" {
		t.Fatalf("%#v", out)
	}
}

func TestRevokeByRenamedEntityRemovesRegistryGrant(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := haobsProven("light.kitchen", "light.kitchen_2")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("DELETE /v1/grants/{entity}/{service}", b.revokeGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
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
		t.Fatalf("%#v", out)
	}
}

func TestAuditEventJSONTags(t *testing.T) {
	b := NewBroker("http://127.0.0.1:9")
	b.mu.Lock()
	b.recordLocked("propose_received", Proposal{ID: "abc", EntityID: "light.kitchen", Service: "light.turn_on"}, "")
	b.mu.Unlock()
	rec := httptest.NewRecorder()
	b.listAudit(rec, httptest.NewRequest(http.MethodGet, "/v1/audit", nil))
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	row := out["audit"].([]any)[0].(map[string]any)
	for _, k := range []string{"id", "at", "action", "proposal_id", "entity_id", "service"} {
		if _, ok := row[k]; !ok {
			t.Fatalf("missing %s in %#v", k, row)
		}
	}
}
