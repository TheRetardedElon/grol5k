package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var eligible = map[string]string{
	"light.turn_on":  "light",
	"light.turn_off": "light",
	"light.toggle":   "light",
	"switch.turn_on": "switch",
	"switch.turn_off": "switch",
	"switch.toggle":  "switch",
}

type Proposal struct {
	ID              string `json:"id"`
	Service         string `json:"service"`
	EntityID        string `json:"entity_id"`
	Reason          string `json:"reason,omitempty"`
	Decision        string `json:"decision"`
	Error           string `json:"error,omitempty"`
	RequiresConfirm bool   `json:"requires_confirm"`
	MutationCapable bool   `json:"mutation_capable"`
	ApplyEnabled    bool   `json:"apply_enabled"`
	Untrusted       bool   `json:"untrusted"`
	CreatedAt       string `json:"created_at"`
}

type Broker struct {
	haobs string
	mu    sync.Mutex
	items map[string]Proposal
	hc    *http.Client
}

func NewBroker(haobs string) *Broker {
	return &Broker{
		haobs: strings.TrimRight(haobs, "/"),
		items: map[string]Proposal{},
		hc:    &http.Client{Timeout: 8 * time.Second},
	}
}

func ListenAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("broker must bind loopback, got %s", addr)
	}
	return nil
}

func (b *Broker) ListenAndServe(addr string) error {
	if err := ListenAddr(addr); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", b.health)
	mux.HandleFunc("POST /v1/propose", b.propose)
	mux.HandleFunc("GET /v1/proposals", b.list)
	mux.HandleFunc("GET /v1/proposals/{id}", b.get)
	mux.HandleFunc("POST /v1/proposals/{id}/apply", b.apply)
	return http.ListenAndServe(addr, mux)
}

func (b *Broker) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "service": "grol-action-broker",
		"apply_enabled": false, "mutation_capable": false,
	})
}

type proposeReq struct {
	Service  string `json:"service"`
	EntityID string `json:"entity_id"`
	Reason   string `json:"reason"`
}

func (b *Broker) propose(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "bad_body"})
		return
	}
	var req proposeReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	req.Service = strings.TrimSpace(req.Service)
	req.EntityID = strings.TrimSpace(req.EntityID)
	p := b.evaluate(req)
	b.mu.Lock()
	b.items[p.ID] = p
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "untrusted": true, "mutation_capable": false, "proposal": p,
	})
}

func (b *Broker) evaluate(req proposeReq) Proposal {
	p := Proposal{
		ID: newID(), Service: req.Service, EntityID: req.EntityID, Reason: req.Reason,
		Untrusted: true, MutationCapable: false, ApplyEnabled: false,
		RequiresConfirm: true, Decision: "denied",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	wantDomain, ok := eligible[req.Service]
	if !ok {
		p.Error = "ineligible_service"
		return p
	}
	if req.EntityID == "" {
		p.Error = "missing_entity"
		return p
	}
	domain, _, _ := strings.Cut(req.EntityID, ".")
	if domain != wantDomain {
		p.Error = "entity_service_mismatch"
		return p
	}
	snap, err := b.house()
	if err != nil {
		p.Error = "haobs_unavailable"
		return p
	}
	if !entityExists(snap, req.EntityID) {
		p.Error = "unknown_entity"
		return p
	}
	p.Error = "not_granted"
	return p
}

func (b *Broker) house() (map[string]any, error) {
	resp, err := b.hc.Get(b.haobs + "/v1/snapshot")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var snap map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&snap); err != nil {
		return nil, err
	}
	if !snapshotHealthy(snap) {
		return nil, fmt.Errorf("haobs unhealthy")
	}
	return snap, nil
}

func snapshotHealthy(snap map[string]any) bool {
	if snap == nil {
		return false
	}
	if ok, exists := snap["ok"].(bool); exists && !ok {
		return false
	}
	switch snap["status"] {
	case "ha_unreachable", "ha_error", "degraded":
		return false
	}
	return true
}

func entityExists(snap map[string]any, id string) bool {
	switch devices := snap["devices"].(type) {
	case []any:
		for _, item := range devices {
			m, _ := item.(map[string]any)
			if m["entity_id"] == id {
				return true
			}
		}
	case []map[string]any:
		for _, m := range devices {
			if m["entity_id"] == id {
				return true
			}
		}
	}
	return false
}

func (b *Broker) list(w http.ResponseWriter, _ *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Proposal, 0, len(b.items))
	for _, p := range b.items {
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proposals": out})
}

func (b *Broker) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b.mu.Lock()
	p, ok := b.items[id]
	b.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proposal": p})
}

func (b *Broker) apply(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok": false, "error": "execute_disabled", "mutation_capable": false,
		"decision": "denied",
		"message":  "M4.0 broker records proposals only. Apply/HA service calls are not enabled.",
	})
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
