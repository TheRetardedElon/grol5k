package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (c *Client) operatorHeaders(req *http.Request) {
	if strings.TrimSpace(c.OperatorToken) == "" {
		return
	}
	req.Header.Set("X-GROL-Actor", "operator")
	req.Header.Set("X-GROL-Operator-Token", c.OperatorToken)
}

func (c *Client) decodeBroker(resp *http.Response) map[string]any {
	var out map[string]any
	if resp != nil && resp.Body != nil {
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&out)
	}
	if out == nil {
		out = map[string]any{}
	}
	out["mutation_capable"] = false
	out["untrusted"] = true
	return out
}

func (c *Client) post(path string, body any) map[string]any {
	raw, _ := json.Marshal(body)
	resp, err := c.hc.Post(c.broker+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	defer resp.Body.Close()
	return c.decodeBroker(resp)
}

func (c *Client) operatorPost(path string, body any) map[string]any {
	if strings.TrimSpace(c.OperatorToken) == "" {
		return map[string]any{
			"ok": false, "error": "operator_required", "mutation_capable": false, "untrusted": true,
			"message": "grant/confirm require grolctl --operator-token; Bot context cannot use them",
		}
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.broker+path, bytes.NewReader(raw))
	if err != nil {
		return map[string]any{"ok": false, "error": "bad_request"}
	}
	req.Header.Set("Content-Type", "application/json")
	c.operatorHeaders(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	defer resp.Body.Close()
	return c.decodeBroker(resp)
}

func (c *Client) Grants() map[string]any {
	out, err := c.get(c.broker + "/v1/grants")
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	out["mutation_capable"] = false
	out["untrusted"] = true
	return out
}

func (c *Client) GrantAdd(entity, service string) map[string]any {
	return c.operatorPost("/v1/grants", map[string]string{"entity_id": entity, "service": service})
}

func (c *Client) GrantRevoke(entity, service string) map[string]any {
	if strings.TrimSpace(c.OperatorToken) == "" {
		return map[string]any{"ok": false, "error": "operator_required", "mutation_capable": false, "untrusted": true}
	}
	req, _ := http.NewRequest(http.MethodDelete, c.broker+"/v1/grants/"+entity+"/"+service, nil)
	c.operatorHeaders(req)
	resp, err := c.hc.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	defer resp.Body.Close()
	return c.decodeBroker(resp)
}

func (c *Client) ProposalGet(id string) map[string]any {
	out, err := c.get(c.broker + "/v1/proposals/" + id)
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	out["mutation_capable"] = false
	out["untrusted"] = true
	return out
}

func (c *Client) ProposalConfirm(id string) map[string]any {
	got := c.ProposalGet(id)
	digest := ""
	if p, ok := got["proposal"].(map[string]any); ok {
		digest, _ = p["confirm_digest"].(string)
	}
	if digest == "" {
		return map[string]any{"ok": false, "error": "confirm_digest_missing", "mutation_capable": false, "untrusted": true}
	}
	return c.operatorPost("/v1/proposals/"+id+"/confirm", map[string]string{"digest": digest})
}

func (c *Client) Audit() map[string]any {
	out, err := c.get(c.broker + "/v1/audit")
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	out["mutation_capable"] = false
	out["untrusted"] = true
	return out
}
