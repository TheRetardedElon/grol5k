package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	healthdMaxResponse = 64 * 1024
	hostdTimeout       = 3 * time.Second
)

type Healthd struct {
	hostSocket string
}

type hostRequest struct {
	ID     string         `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

type hostResponse struct {
	ID     any            `json:"id,omitempty"`
	OK     bool           `json:"ok"`
	Error  string         `json:"error,omitempty"`
	Result map[string]any `json:"result,omitempty"`
}

func NewHealthd(hostSocket string) *Healthd {
	return &Healthd{hostSocket: hostSocket}
}

func (h *Healthd) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /v1/snapshot", h.snapshot)
	return http.ListenAndServe(addr, mux)
}

func (h *Healthd) health(w http.ResponseWriter, _ *http.Request) {
	_, err := h.hostCall("grol.system.status", nil)
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"status":  status,
		"service": "grol-healthd",
		"mode":    "read-only",
		"hostd": map[string]any{
			"reachable": err == nil,
		},
	})
}

func (h *Healthd) snapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Snapshot())
}

func (h *Healthd) Snapshot() map[string]any {
	type readSpec struct {
		key    string
		method string
		params map[string]any
	}
	reads := []readSpec{
		{key: "system", method: "grol.system.status"},
		{key: "network", method: "grol.network.status"},
		{key: "hardware", method: "grol.hardware.status"},
		{key: "update", method: "grol.update.status"},
		{key: "services", method: "grol.service.status", params: map[string]any{}},
	}

	out := map[string]any{
		"ok":               true,
		"status":           "ok",
		"service":          "grol-healthd",
		"source":           "grol-hostd",
		"hostd_reachable":  true,
		"captured_at":      time.Now().UTC(),
		"mutation_capable": false,
	}
	errs := map[string]string{}
	successes := 0

	for _, spec := range reads {
		result, err := h.hostCall(spec.method, spec.params)
		if err != nil {
			errs[spec.key] = classifyHostError(err)
			continue
		}
		successes++
		out[spec.key] = result
	}

	if successes != len(reads) {
		out["ok"] = false
		out["status"] = "degraded"
		out["errors"] = errs
	}
	if successes == 0 {
		out["hostd_reachable"] = false
	}

	return out
}

func (h *Healthd) hostCall(method string, params map[string]any) (map[string]any, error) {
	if !isReadMethod(method) {
		return nil, errors.New("method_not_allowed")
	}
	if params == nil {
		params = map[string]any{}
	}

	conn, err := net.DialTimeout("unix", h.hostSocket, hostdTimeout)
	if err != nil {
		return nil, fmt.Errorf("hostd_unreachable: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(hostdTimeout))

	req := hostRequest{
		ID:     "healthd",
		Method: method,
		Params: params,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("hostd_write: %w", err)
	}

	reader := bufio.NewReader(io.LimitReader(conn, healthdMaxResponse))
	line, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("hostd_read: %w", err)
	}
	if len(line) == 0 {
		return nil, errors.New("hostd_empty_response")
	}

	var resp hostResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("hostd_decode: %w", err)
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "hostd_error"
		}
		return nil, errors.New(resp.Error)
	}
	if resp.Result == nil {
		resp.Result = map[string]any{}
	}
	return resp.Result, nil
}

func isReadMethod(method string) bool {
	switch method {
	case "grol.system.status",
		"grol.update.status",
		"grol.network.status",
		"grol.hardware.status",
		"grol.service.status":
		return true
	default:
		return false
	}
}

func classifyHostError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	switch {
	case strings.Contains(text, "method_not_allowed"):
		return "method_not_allowed"
	case strings.Contains(text, "forbidden"):
		return "forbidden"
	case strings.Contains(text, "unreachable"):
		return "unreachable"
	case strings.Contains(text, "deadline") || strings.Contains(text, "timeout"):
		return "timeout"
	default:
		return "hostd_error"
	}
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
