package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type RegistryEntry struct {
	EntityID   string
	RegistryID string
	Platform   string
	Domain     string
}

func (o *Observer) fetchRegistry() (map[string]RegistryEntry, error) {
	if o.lookupRegistry != nil {
		return o.lookupRegistry()
	}
	return o.fetchRegistryWS()
}

func (o *Observer) fetchRegistryWS() (map[string]RegistryEntry, error) {
	u, err := url.Parse(o.baseURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = "/api/websocket"
	u.RawQuery = ""
	conn, err := dialWS(u, o.client.Timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg, err := readWSText(conn)
	if err != nil {
		return nil, err
	}
	var hello map[string]any
	if err := json.Unmarshal(msg, &hello); err != nil {
		return nil, err
	}
	if hello["type"] != "auth_required" {
		return nil, fmt.Errorf("unexpected hello %v", hello["type"])
	}
	auth, _ := json.Marshal(map[string]string{"type": "auth", "access_token": o.token})
	if err := writeWSText(conn, auth); err != nil {
		return nil, err
	}
	msg, err = readWSText(conn)
	if err != nil {
		return nil, err
	}
	var authed map[string]any
	if err := json.Unmarshal(msg, &authed); err != nil {
		return nil, err
	}
	if authed["type"] != "auth_ok" {
		return nil, fmt.Errorf("auth failed: %v", authed["type"])
	}
	req, _ := json.Marshal(map[string]any{"id": 1, "type": "config/entity_registry/list"})
	if err := writeWSText(conn, req); err != nil {
		return nil, err
	}
	msg, err = readWSText(conn)
	if err != nil {
		return nil, err
	}
	var result struct {
		Type    string           `json:"type"`
		Success bool             `json:"success"`
		Result  []map[string]any `json:"result"`
	}
	if err := json.Unmarshal(msg, &result); err != nil {
		return nil, err
	}
	if result.Type != "result" || !result.Success {
		return nil, fmt.Errorf("registry list failed")
	}
	out := map[string]RegistryEntry{}
	for _, row := range result.Result {
		eid, _ := row["entity_id"].(string)
		if eid == "" {
			continue
		}
		domain, _, _ := strings.Cut(eid, ".")
		regID, _ := row["id"].(string)
		platform, _ := row["platform"].(string)
		out[eid] = RegistryEntry{EntityID: eid, RegistryID: regID, Platform: platform, Domain: domain}
	}
	return out, nil
}
