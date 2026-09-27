package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestNotGrantedVsPendingConfirm(t *testing.T) {
	hs := haobsOK("light.kitchen")
	defer hs.Close()
	b := NewBroker(hs.URL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/propose", b.propose)
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("POST /v1/proposals/{id}/confirm", b.confirm)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p := out["proposal"].(map[string]any)
	if p["state"] != "not_granted" || p["decision"] != "denied" {
		t.Fatalf("want not_granted %#v", p)
	}

	code, gout := postJSON(t, srv.URL+"/v1/grants", map[string]string{"entity_id": "light.kitchen", "service": "light.turn_on"})
	if code != 200 || gout["ok"] != true {
		t.Fatalf("grant: %d %#v", code, gout)
	}

	_, out = postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	p = out["proposal"].(map[string]any)
	if p["state"] != "pending_confirmation" || p["decision"] != "confirmation_required" {
		t.Fatalf("want pending %#v", p)
	}
	id, _ := p["id"].(string)
	_, cout := postJSON(t, srv.URL+"/v1/proposals/"+id+"/confirm", map[string]string{})
	cp := cout["proposal"].(map[string]any)
	if cp["state"] != "confirmed" || cp["apply_enabled"] != false {
		t.Fatalf("confirm %#v", cout)
	}
}

func TestConfirmExpires(t *testing.T) {
	hs := haobsOK("light.kitchen")
	defer hs.Close()
	b := NewBroker(hs.URL)
	now := time.Date(2026, 9, 27, 15, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	b.grants[grantKey("light.kitchen", "light.turn_on")] = Grant{
		EntityID: "light.kitchen", Service: "light.turn_on", RequiresConfirm: true,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/propose", b.propose)
	mux.HandleFunc("GET /v1/proposals/{id}", b.get)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	_, out := postJSON(t, srv.URL+"/v1/propose", map[string]string{"service": "light.turn_on", "entity_id": "light.kitchen"})
	id := out["proposal"].(map[string]any)["id"].(string)
	b.now = func() time.Time { return now.Add(16 * time.Minute) }
	resp, err := http.Get(srv.URL + "/v1/proposals/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	p := got["proposal"].(map[string]any)
	if p["state"] != "expired" {
		t.Fatalf("%#v", p)
	}
}

func TestListenRejectsNonLoopback(t *testing.T) {
	if err := ListenAddr("0.0.0.0:8785"); err == nil {
		t.Fatal("LAN bind must fail closed")
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
