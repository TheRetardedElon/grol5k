package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	ha     string
	obs    string
	bot    string
	broker string
	hc     *http.Client
}

func NewClient(ha, obs, bot string) *Client {
	return NewClientWithBroker(ha, obs, bot, "http://127.0.0.1:8785")
}

func NewClientWithBroker(ha, obs, bot, broker string) *Client {
	return &Client{
		ha:     strings.TrimRight(ha, "/"),
		obs:    strings.TrimRight(obs, "/"),
		bot:    strings.TrimRight(bot, "/"),
		broker: strings.TrimRight(broker, "/"),
		hc:     &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) get(url string) (map[string]any, error) {
	resp, err := c.hc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func reachable(err error, payload map[string]any) bool {
	return err == nil && payload != nil
}

func (c *Client) Status() map[string]any {
	ha, haErr := c.get(c.ha + "/health")
	host, hostErr := c.get(c.obs + "/health")
	bot, botErr := c.get(c.bot + "/health")
	brk, brkErr := c.get(c.broker + "/health")
	return map[string]any{
		"ok":               true,
		"service":          "grolctl",
		"mode":             "read-plus-propose",
		"mutation_capable": false,
		"untrusted":        true,
		"haobs":            reachable(haErr, ha),
		"healthd":          reachable(hostErr, host),
		"bot":              reachable(botErr, bot),
		"broker":           reachable(brkErr, brk),
	}
}

func (c *Client) System() map[string]any {
	snap, err := c.get(c.obs + "/v1/snapshot")
	if err != nil {
		return map[string]any{"ok": false, "error": "observer_unavailable", "mutation_capable": false, "untrusted": true}
	}
	return map[string]any{"ok": true, "mutation_capable": false, "untrusted": true, "snapshot": snap}
}

func (c *Client) House() map[string]any {
	snap, err := c.get(c.ha + "/v1/snapshot")
	if err != nil {
		return map[string]any{"ok": false, "error": "haobs_unavailable", "mutation_capable": false, "untrusted": true}
	}
	snap["mutation_capable"] = false
	snap["untrusted"] = true
	return snap
}

func (c *Client) Devices() map[string]any { return c.House() }

func (c *Client) Device(id string) map[string]any {
	house := c.House()
	if ok, _ := house["ok"].(bool); !ok {
		return house
	}
	if raw, ok := house["devices"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			if m["entity_id"] == id {
				return map[string]any{
					"ok": true, "mutation_capable": false, "untrusted": true,
					"status": house["status"], "service": house["service"],
					"captured_at": house["captured_at"], "device": m,
				}
			}
		}
	}
	if maps, ok := house["devices"].([]map[string]any); ok {
		for _, m := range maps {
			if m["entity_id"] == id {
				return map[string]any{
					"ok": true, "mutation_capable": false, "untrusted": true,
					"status": house["status"], "service": house["service"],
					"captured_at": house["captured_at"], "device": m,
				}
			}
		}
	}
	return map[string]any{"ok": false, "error": "not_found", "entity_id": id, "mutation_capable": false, "untrusted": true}
}

func (c *Client) Activity() map[string]any {
	payload, err := c.get(c.bot + "/v1/activity")
	if err != nil {
		return map[string]any{"ok": false, "error": "bot_unavailable", "mutation_capable": false, "untrusted": true}
	}
	return map[string]any{"ok": true, "mutation_capable": false, "untrusted": true, "activity": payload["activity"]}
}

func (c *Client) Propose(service, entity string) map[string]any {
	body, _ := json.Marshal(map[string]string{
		"service":   service,
		"entity_id": entity,
		"reason":    "grolctl propose",
	})
	resp, err := c.hc.Post(c.broker+"/v1/propose", "application/json", bytes.NewReader(body))
	if err != nil {
		return map[string]any{
			"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true,
			"message": "start grol-action-broker on 127.0.0.1:8785",
		}
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&out)
	if out == nil {
		out = map[string]any{}
	}
	out["mutation_capable"] = false
	out["untrusted"] = true
	return out
}
