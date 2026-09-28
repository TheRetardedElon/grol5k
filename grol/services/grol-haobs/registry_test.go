package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegistryJoinProvesIdentity(t *testing.T) {
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/states" {
			http.Error(w, "nope", http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"entity_id": "light.kitchen", "state": "off", "attributes": map[string]any{"friendly_name": "Kitchen"}},
		})
	}))
	defer ha.Close()
	o := NewObserver(ha.URL, "secret-token", "/no/such")
	o.lookupRegistry = func() (map[string]RegistryEntry, error) {
		return map[string]RegistryEntry{
			"light.kitchen": {EntityID: "light.kitchen", RegistryID: "reg-kitchen", Platform: "hue", Domain: "light"},
		}, nil
	}
	snap := o.Snapshot()
	devices, _ := snap["devices"].([]map[string]any)
	if snap["registry_status"] != "ok" || devices[0]["identity_proven"] != true {
		t.Fatalf("%#v", snap)
	}
	if devices[0]["registry_id"] != "reg-kitchen" || devices[0]["platform"] != "hue" {
		t.Fatalf("%#v", devices[0])
	}
	if snap["mutation_capable"] != false {
		t.Fatal("M4.2 must not enable mutation")
	}
}

func TestRegistryFailureStillReadsStates(t *testing.T) {
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"entity_id": "switch.porch", "state": "on", "attributes": map[string]any{"friendly_name": "Porch"}},
		})
	}))
	defer ha.Close()
	o := NewObserver(ha.URL, "secret-token", "/no/such")
	o.lookupRegistry = func() (map[string]RegistryEntry, error) { return nil, io.ErrUnexpectedEOF }
	snap := o.Snapshot()
	if snap["ok"] != true || snap["status"] != "ok" {
		t.Fatalf("states should survive registry miss %#v", snap)
	}
	if snap["registry_status"] != "unavailable" {
		t.Fatalf("%#v", snap)
	}
	devices, _ := snap["devices"].([]map[string]any)
	if devices[0]["identity_proven"] != false {
		t.Fatalf("%#v", devices[0])
	}
}
