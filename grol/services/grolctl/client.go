package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	ha  string
	obs string
	bot string
	hc  *http.Client
}

func NewClient(ha, obs, bot string) *Client {
	return &Client{
		ha:  strings.TrimRight(ha, "/"),
		obs: strings.TrimRight(obs, "/"),
		bot: strings.TrimRight(bot, "/"),
		hc:  &http.Client{Timeout: 8 * time.Second},
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

	haobsProcess := reachable(haHealthErr, haHealth)
	houseReachable := payloadOK(houseErr, house)
	healthdProcess := reachable(hostHealthErr, hostHealth)
	systemReachable := payloadOK(systemErr, system)
	botProcess := reachable(botHealthErr, botHealth)

	status := "ok"
	if !haobsProcess || !houseReachable || !healthdProcess || !systemReachable || !botProcess {
		status = "degraded"
	}

	return map[string]any{
		"ok":               true,
		"status":           status,
		"service":          "grolctl",
		"mode":             "read-only",
		"mutation_capable": false,
		"untrusted":        true,
		"haobs":            haobsProcess,
		"house_reachable":  houseReachable,
		"healthd":          healthdProcess,
		"system_reachable": systemReachable,
		"bot":              botProcess,
	}
}

func (c *Client) System() map[string]any {
	snap, err := c.get(c.obs + "/v1/snapshot")
	if err != nil {
		return map[string]any{
			"ok":               false,
			"error":            "observer_unavailable",
			"mutation_capable": false,
			"untrusted":        true,
		}
	}
	snap["mutation_capable"] = false
	snap["untrusted"] = true
	return snap
}

func (c *Client) House() map[string]any {
	snap, err := c.get(c.ha + "/v1/snapshot")
	if err != nil {
		return map[string]any{
			"ok":               false,
			"error":            "haobs_unavailable",
			"mutation_capable": false,
			"untrusted":        true,
		}
	}
	snap["mutation_capable"] = false
	snap["untrusted"] = true
	return snap
}

func (c *Client) Devices() map[string]any {
	return c.House()
}

func (c *Client) Device(id string) map[string]any {
	house := c.House()
	if ok, _ := house["ok"].(bool); !ok {
		return house
	}
	raw, _ := house["devices"].([]any)
	for _, item := range raw {
		m, _ := item.(map[string]any)
		eid, _ := m["entity_id"].(string)
		if eid == id {
			return map[string]any{
				"ok":               true,
				"mutation_capable": false,
				"untrusted":        true,
				"device":           m,
			}
		}
	}
	// also accept []map from tests
	if maps, ok := house["devices"].([]map[string]any); ok {
		for _, m := range maps {
			if m["entity_id"] == id {
				return map[string]any{
					"ok":               true,
					"mutation_capable": false,
					"untrusted":        true,
					"device":           m,
				}
			}
		}
	}
	return map[string]any{
		"ok":               false,
		"error":            "not_found",
		"entity_id":        id,
		"mutation_capable": false,
		"untrusted":        true,
	}
}

func (c *Client) Activity() map[string]any {
	payload, err := c.get(c.bot + "/v1/activity")
	if err != nil {
		return map[string]any{
			"ok":               false,
			"error":            "bot_unavailable",
			"mutation_capable": false,
			"untrusted":        true,
		}
	}
	payload["mutation_capable"] = false
	payload["untrusted"] = true
	return payload
}
