package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const confirmTTL = 15 * time.Minute

var eligible = map[string]string{
	"light.turn_on":   "light",
	"light.turn_off":  "light",
	"light.toggle":    "light",
	"switch.turn_on":  "switch",
	"switch.turn_off": "switch",
	"switch.toggle":   "switch",
}

type Proposal struct {
	ID              string `json:"id"`
	Service         string `json:"service"`
	EntityID        string `json:"entity_id"`
	Reason          string `json:"reason,omitempty"`
	State           string `json:"state"`
	Decision        string `json:"decision"`
	Error           string `json:"error,omitempty"`
	RequiresConfirm bool   `json:"requires_confirm"`
	MutationCapable bool   `json:"mutation_capable"`
	ApplyEnabled    bool   `json:"apply_enabled"`
	Untrusted       bool   `json:"untrusted"`
	CreatedAt       string `json:"created_at"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	ConfirmedAt     string `json:"confirmed_at,omitempty"`
}

type Grant struct {
	EntityID        string `json:"entity_id"`
	Service         string `json:"service"`
	RequiresConfirm bool   `json:"requires_confirm"`
	CreatedAt       string `json:"created_at"`
}

type AuditEvent struct {
	ID       string `json:"id"`
	At       string `json:"at"`
	Action   string `json:"action"`
	Proposal string `json:"proposal_id,omitempty"`
	EntityID string `json:"entity_id,omitempty"`
	Service  string `json:"service,omitempty"`
	State    string `json:"state,omitempty"`
	Decision string `json:"decision,omitempty"`
	Error    string `json:"error,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type Broker struct {
	haobs  string
	now    func() time.Time
	mu     sync.Mutex
	items  map[string]Proposal
	grants map[string]Grant
	audit  []AuditEvent
	hc     *http.Client
}

func NewBroker(haobs string) *Broker {
	return &Broker{
		haobs:  strings.TrimRight(haobs, "/"),
		now:    func() time.Time { return time.Now().UTC() },
		items:  map[string]Proposal{},
		grants: map[string]Grant{},
		hc:     &http.Client{Timeout: 8 * time.Second},
	}
}

func grantKey(entity, service string) string { return entity + "\x00" + service }

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
	mux.HandleFunc("POST /v1/proposals/{id}/confirm", b.confirm)
	mux.HandleFunc("POST /v1/proposals/{id}/apply", b.apply)
	mux.HandleFunc("GET /v1/grants", b.listGrants)
	mux.HandleFunc("POST /v1/grants", b.addGrant)
	mux.HandleFunc("DELETE /v1/grants/{entity}/{service}", b.revokeGrant)
	mux.HandleFunc("GET /v1/audit", b.listAudit)
	return http.ListenAndServe(addr, mux)
}

func (b *Broker) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "service": "grol-action-broker",
		"apply_enabled": false, "mutation_capable": false, "mode": "m4.1-grants-confirm",
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
	b.recordLocked("propose_received", p, "")
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "untrusted": true, "mutation_capable": false, "proposal": p,
	})
}

func (b *Broker) evaluate(req proposeReq) Proposal {
	now := b.now()
	p := Proposal{
		ID: newID(), Service: req.Service, EntityID: req.EntityID, Reason: req.Reason,
		Untrusted: true, MutationCapable: false, ApplyEnabled: false,
		RequiresConfirm: true, State: "denied", Decision: "denied",
		CreatedAt: now.Format(time.RFC3339),
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
	b.mu.Lock()
	g, granted := b.grants[grantKey(req.EntityID, req.Service)]
	b.mu.Unlock()
	if !granted {
		p.State = "not_granted"
		p.Decision = "denied"
		p.Error = "not_granted"
		return p
	}
	p.RequiresConfirm = g.RequiresConfirm
	p.State = "pending_confirmation"
	p.Decision = "confirmation_required"
	p.Error = ""
	p.ExpiresAt = now.Add(confirmTTL).Format(time.RFC3339)
	return p
}
