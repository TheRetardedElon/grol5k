package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

func (b *Broker) refreshLocked(p Proposal) Proposal {
	if p.State != "pending_confirmation" || p.ExpiresAt == "" {
		return p
	}
	exp, err := time.Parse(time.RFC3339, p.ExpiresAt)
	if err != nil {
		return p
	}
	if !b.now().Before(exp) {
		p.State = "expired"
		p.Decision = "denied"
		p.Error = "expired"
		b.items[p.ID] = p
		b.recordLocked("proposal_expired", p, "")
	}
	return p
}

func (b *Broker) list(w http.ResponseWriter, _ *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Proposal, 0, len(b.items))
	for _, p := range b.items {
		out = append(out, b.refreshLocked(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proposals": out})
}

func (b *Broker) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b.mu.Lock()
	p, ok := b.items[id]
	if ok {
		p = b.refreshLocked(p)
	}
	b.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "not_found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proposal": p, "mutation_capable": false})
}

func (b *Broker) confirm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b.mu.Lock()
	defer b.mu.Unlock()
	p, ok := b.items[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "not_found"})
		return
	}
	p = b.refreshLocked(p)
	if p.State != "pending_confirmation" {
		writeJSON(w, http.StatusConflict, map[string]any{
			"ok": false, "error": "not_confirmable", "proposal": p,
			"message": "only pending_confirmation proposals can be confirmed",
		})
		return
	}
	p.State = "confirmed"
	p.Decision = "confirmed"
	p.ConfirmedAt = b.now().Format(time.RFC3339)
	p.ApplyEnabled = false
	p.MutationCapable = false
	b.items[id] = p
	b.recordLocked("proposal_confirmed", p, "human confirmation; apply still disabled")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "proposal": p, "mutation_capable": false, "apply_enabled": false,
	})
}

func (b *Broker) apply(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok": false, "error": "execute_disabled", "mutation_capable": false,
		"decision": "denied",
		"message":  "M4.1 records grants/confirmation only. HA service calls stay disabled.",
	})
}

type grantReq struct {
	EntityID string `json:"entity_id"`
	Service  string `json:"service"`
}

func (b *Broker) addGrant(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "bad_body"})
		return
	}
	var req grantReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid_json"})
		return
	}
	req.EntityID = strings.TrimSpace(req.EntityID)
	req.Service = strings.TrimSpace(req.Service)
	if _, ok := eligible[req.Service]; !ok || req.EntityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "ineligible_grant"})
		return
	}
	g := Grant{
		EntityID: req.EntityID, Service: req.Service,
		RequiresConfirm: true, CreatedAt: b.now().Format(time.RFC3339),
	}
	b.mu.Lock()
	b.grants[grantKey(g.EntityID, g.Service)] = g
	b.recordLocked("grant_added", Proposal{EntityID: g.EntityID, Service: g.Service}, "operator grant; confirm still required")
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grant": g})
}

func (b *Broker) listGrants(w http.ResponseWriter, _ *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Grant, 0, len(b.grants))
	for _, g := range b.grants {
		out = append(out, g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grants": out})
}

func (b *Broker) revokeGrant(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	service := r.PathValue("service")
	b.mu.Lock()
	delete(b.grants, grantKey(entity, service))
	b.recordLocked("grant_revoked", Proposal{EntityID: entity, Service: service}, "")
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (b *Broker) listAudit(w http.ResponseWriter, _ *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "audit": b.audit})
}

func (b *Broker) recordLocked(action string, p Proposal, detail string) {
	b.audit = append(b.audit, AuditEvent{
		ID: newID(), At: b.now().Format(time.RFC3339), Action: action,
		Proposal: p.ID, EntityID: p.EntityID, Service: p.Service,
		State: p.State, Decision: p.Decision, Error: p.Error, Detail: detail,
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
