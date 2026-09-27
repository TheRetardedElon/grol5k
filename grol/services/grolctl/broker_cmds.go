package main

import (
	"encoding/json"
	"io"
	"net/http"
)

func (c *Client) post(path string, body any) map[string]any {
	raw, _ := json.Marshal(body)
	resp, err := c.hc.Post(c.broker+path, "application/json", bytesReader(raw))
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
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

func bytesReader(raw []byte) *readCloserBuf { return &readCloserBuf{b: raw} }

type readCloserBuf struct {
	b []byte
	n int
}

func (r *readCloserBuf) Read(p []byte) (int, error) {
	if r.n >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.n:])
	r.n += n
	return n, nil
}
func (r *readCloserBuf) Close() error { return nil }

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
	return c.post("/v1/grants", map[string]string{"entity_id": entity, "service": service})
}

func (c *Client) GrantRevoke(entity, service string) map[string]any {
	req, _ := http.NewRequest(http.MethodDelete, c.broker+"/v1/grants/"+entity+"/"+service, nil)
	resp, err := c.hc.Do(req)
	if err != nil {
		return map[string]any{"ok": false, "error": "broker_unavailable", "mutation_capable": false, "untrusted": true}
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out == nil {
		out = map[string]any{"ok": true}
	}
	out["mutation_capable"] = false
	return out
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
	return c.post("/v1/proposals/"+id+"/confirm", map[string]string{})
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
