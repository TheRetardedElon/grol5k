package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func (b *Broker) apply(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok": false, "error": "execute_disabled", "mutation_capable": false,
		"decision": "denied",
		"message":  "M4.3 records identity-authorized grants only. HA service calls stay disabled.",
	})
}

type grantReq struct {
	EntityID   string `json:"entity_id"`
	Service    string `json:"service"`
	RegistryID string `json:"registry_id"`
	Platform   string `json:"platform"`
}

func (b *Broker) addGrant(w http.ResponseWriter, r *http.Request) {
	if !isOperator(r) {
		writeOperatorDenied(w)
		return
	}
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
	domain, _, _ := strings.Cut(req.EntityID, ".")
	g := Grant{
		EntityID: req.EntityID, Service: req.Service, Domain: domain,
		RequiresConfirm: true, CreatedAt: b.now().Format(time.RFC3339),
	}
	snap, err := b.house()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "haobs_unavailable"})
		return
	}
	if !entityExists(snap, req.EntityID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unknown_entity"})
		return
	}
	regID, liveDomain, platform, proven := deviceIdentity(findDevice(snap, req.EntityID))
	if liveDomain != "" {
		g.Domain = liveDomain
	}
	g.RegistryID = regID
	g.Platform = platform
	g.Authorizing = proven
	b.mu.Lock()
	b.grants[grantKey(g.EntityID, g.Service)] = g
	if g.RegistryID != "" {
		b.grants[grantKey("reg:"+g.RegistryID, g.Service)] = g
	}
	b.recordLocked("grant_added", Proposal{EntityID: g.EntityID, Service: g.Service}, "operator grant")
	b.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grant": g})
}

func (b *Broker) listGrants(w http.ResponseWriter, _ *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()
	seen := map[string]struct{}{}
	out := make([]Grant, 0, len(b.grants))
	for _, g := range b.grants {
		id := g.RegistryID + "\x00" + g.Service + "\x00" + g.EntityID
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, g)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "grants": out})
}

func (b *Broker) revokeGrant(w http.ResponseWriter, r *http.Request) {
	if !isOperator(r) {
		writeOperatorDenied(w)
		return
	}
	entity := r.PathValue("entity")
	service := r.PathValue("service")
	liveReg := ""
	if snap, err := b.house(); err == nil {
		id, _, _, proven := deviceIdentity(findDevice(snap, entity))
		if proven {
			liveReg = id
		}
	}
	b.mu.Lock()
	g, ok := b.lookupGrantLocked(entity, service, liveReg)
	if ok {
		delete(b.grants, grantKey(g.EntityID, g.Service))
		if g.RegistryID != "" {
			delete(b.grants, grantKey("reg:"+g.RegistryID, g.Service))
		}
	}
	delete(b.grants, grantKey(entity, service))
	if liveReg != "" {
		delete(b.grants, grantKey("reg:"+liveReg, service))
	}
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
