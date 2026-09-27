package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func findDevice(snap map[string]any, id string) map[string]any {
	switch devices := snap["devices"].(type) {
	case []any:
		for _, item := range devices {
			m, _ := item.(map[string]any)
			if m["entity_id"] == id {
				return m
			}
		}
	case []map[string]any:
		for _, m := range devices {
			if m["entity_id"] == id {
				return m
			}
		}
	}
	return map[string]any{}
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
	if !isOperator(r) {
		writeOperatorDenied(w)
		return
	}
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
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "not_confirmable", "proposal": p})
		return
	}
	var req struct {
		Digest string `json:"digest"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 16*1024)).Decode(&req)
	if hdr := r.Header.Get("X-GROL-Confirm-Digest"); hdr != "" {
		req.Digest = hdr
	}
	if p.ConfirmDigest == "" || req.Digest == "" || req.Digest != p.ConfirmDigest {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "confirm_digest_mismatch", "proposal": p})
		return
	}
	p.State = "confirmed"
	p.Decision = "confirmed"
	p.ConfirmedAt = b.now().Format(time.RFC3339)
	p.ApplyEnabled = false
	p.MutationCapable = false
	b.items[id] = p
	b.recordLocked("proposal_confirmed", p, "human confirmation; apply still disabled")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proposal": p, "mutation_capable": false, "apply_enabled": false})
}
