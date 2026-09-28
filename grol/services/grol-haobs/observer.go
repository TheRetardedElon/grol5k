package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
)

const maxEntities = 200

var allowDomains = map[string]struct{}{
	"light": {}, "switch": {}, "climate": {}, "media_player": {},
	"lock": {}, "binary_sensor": {}, "sensor": {}, "cover": {},
	"weather": {},
}

type Observer struct {
	baseURL        string
	token          string
	client         *http.Client
	lookupRegistry func() (map[string]RegistryEntry, error)
}

func NewObserver(baseURL, envToken, tokenFile string) *Observer {
	token := strings.TrimSpace(envToken)
	if token == "" {
		if b, err := os.ReadFile(tokenFile); err == nil {
			token = strings.TrimSpace(string(b))
		}
	}
	transport := &http.Transport{
		Proxy:               nil,
		ForceAttemptHTTP2:   false,
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: 10 * time.Second,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	}
	return &Observer{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

func (o *Observer) Provisioned() bool { return o.token != "" }

func (o *Observer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", o.health)
	mux.HandleFunc("GET /v1/snapshot", o.snapshot)
	return http.ListenAndServe(addr, mux)
}

func (o *Observer) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "service": "grol-haobs", "mode": "read-only",
		"provisioned": o.Provisioned(), "mutation_capable": false,
	})
}

func (o *Observer) snapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, o.Snapshot())
}

func (o *Observer) Snapshot() map[string]any {
	out := map[string]any{
		"ok": true, "service": "grol-haobs",
		"source": "home-assistant-states+entity-registry",
		"mutation_capable": false, "untrusted": true,
		"captured_at": time.Now().UTC(), "devices": []any{},
	}
	if !o.Provisioned() {
		out["ok"] = false
		out["status"] = "unprovisioned"
		return out
	}

	req, err := http.NewRequest(http.MethodGet, o.baseURL+"/api/states", nil)
	if err != nil {
		out["ok"] = false
		out["status"] = "bad_request"
		out["error"] = err.Error()
		return out
	}
	req.Header.Set("Authorization", "Bearer "+o.token)
	req.Header.Set("Accept", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		out["ok"] = false
		out["status"] = "ha_unreachable"
		out["error"] = err.Error()
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out["ok"] = false
		out["status"] = "ha_error"
		out["ha_status"] = resp.StatusCode
		return out
	}

	var raw []map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&raw); err != nil {
		out["ok"] = false
		out["status"] = "ha_decode"
		out["error"] = err.Error()
		return out
	}

	devices := make([]map[string]any, 0, 32)
	for _, item := range raw {
		eid, _ := item["entity_id"].(string)
		domain, _, _ := strings.Cut(eid, ".")
		if _, ok := allowDomains[domain]; !ok {
			continue
		}
		state, _ := item["state"].(string)
		name := ""
		if attrs, ok := item["attributes"].(map[string]any); ok {
			if fn, ok := attrs["friendly_name"].(string); ok {
				name = sanitizeLabel(fn)
			}
		}
		devices = append(devices, map[string]any{
			"entity_id": eid, "domain": domain, "state": sanitizeLabel(state), "name": name,
			"registry_id": "", "platform": "", "identity_proven": false,
		})
		if len(devices) >= maxEntities {
			break
		}
	}
	reg, err := o.fetchRegistry()
	if err != nil {
		out["registry_status"] = "unavailable"
	} else {
		out["registry_status"] = "ok"
		proven := 0
		for i := range devices {
			eid, _ := devices[i]["entity_id"].(string)
			if entry, ok := reg[eid]; ok {
				devices[i]["registry_id"] = entry.RegistryID
				devices[i]["platform"] = entry.Platform
				if entry.Domain != "" {
					devices[i]["domain"] = entry.Domain
				}
				okID := entry.RegistryID != "" && entry.Platform != "" && devices[i]["domain"] != ""
				devices[i]["identity_proven"] = okID
				if okID {
					proven++
				}
			}
		}
		out["identity_proven_count"] = proven
	}
	out["devices"] = devices
	out["count"] = len(devices)
	out["status"] = "ok"
	return out
}

func sanitizeLabel(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		n++
		if n >= 80 {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
