package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBotCannotGrantOrConfirm(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	b := NewBroker("http://127.0.0.1:9")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/proposals/{id}/confirm", b.confirm)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, out := postJSON(t, srv.URL+"/v1/grants", map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	if code != http.StatusForbidden || out["error"] != "operator_required" {
		t.Fatalf("%d %#v", code, out)
	}
}

func TestDraftGrantDoesNotAuthorize(t *testing.T) {
	t.Setenv("GROL_OPERATOR_TOKEN", "secret-op")
	hs := haobsOK("light.kitchen")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/propose", b.propose)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/grants", bytes.NewReader([]byte(`{"entity_id":"light.kitchen","service":"light.turn_on"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", "secret-op")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var gout map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&gout)
	if gout["grant"].(map[string]any)["authorizing"] != false {
		t.Fatalf("%#v", gout)
	}
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	if out["proposal"].(map[string]any)["error"] != "grant_identity_unproven" {
		t.Fatalf("%#v", out)
	}
}
