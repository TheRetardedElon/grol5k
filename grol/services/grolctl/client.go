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

func payloadOK(err error, payload map[string]any) bool {
	if !reachable(err, payload) {
		return false
	}
	ok, _ := payload["ok"].(bool)
	return ok
}

func (c *Client) Status() map[string]any {
	haHealth, haHealthErr := c.get(c.ha + "/health")
	house, houseErr := c.get(c.ha + "/v1/snapshot")
	hostHealth, hostHealthErr := c.get(c.obs + "/health")
	system, systemErr := c.get(c.obs + "/v1/snapshot")
	botHealth, botHealthErr := c.get(c.bot + "/health")
	brkHealth, brkErr := c.get(c.broker + "/health")

	haobsProcess := reachable(haHealthErr, haHealth)
	houseReachable := payloadOK(houseErr, house)
	healthdProcess := reachable(hostHealthErr, hostHealth)
	systemReachable := payloadOK(systemErr, system)
	botProcess := reachable(botHealthErr, botHealth)
	brokerProcess := reachable(brkErr, brkHealth)

	status := "ok"
	if !haobsProcess || !houseReachable || !healthdProcess || !systemReachable || !botProcess {
		status = "degraded"
	}

	return map[string]any{
		"ok":               true,
		"status":           status,
		"service":          "grolctl",
		"mode":             "read-plus-propose",
		"mutation_capable": false,
		"untrusted":        true,
		"haobs":            haobsProcess,
		"house_reachable":  houseReachable,
		"healthd":          healthdProcess,
		"system_reachable": systemReachable,
		"bot":              botProcess,
		"broker":           brokerProcess,
	}
}

func (c *Client) System() map[string]any {
	snap, err := c.get(c.obs + "/v1/snapshot")
	if err != nil {
		return map[string]any{"ok": false, "error": "observer_unavailable", "mutation_capable": false, "untrusted": true}
	}
	snap["mutation_capable"] = false
	snap["untrusted"] = true
	return snap
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
	env := func(m map[string]any) map[string]any {
		return map[string]any{
			"ok": true, "mutation_capable": false, "untrusted": true,
			"status": house["status"], "service": house["service"],
			"captured_at": house["captured_at"], "device": m,
		}
	}
	if raw, ok := house["devices"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			if m["entity_id"] == id {
				return env(m)
			}
		}
	}
	if maps, ok := house["devices"].([]map[string]any); ok {
		for _, m := range maps {
			if m["entity_id"] == id {
				return env(m)
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
	payload["mutation_capable"] = false
	payload["untrusted"] = true
	return payload
}

func (c *Client) Propose(service, entity string) map[string]any {
	body, _ := json.Marshal(map[string]string{"service": service, "entity_id": entity, "reason": "grolctl propose"})
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
