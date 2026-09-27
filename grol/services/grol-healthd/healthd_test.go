package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func startFakeHostd(t *testing.T, results map[string]map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	sock := filepath.Join(dir, "hostapi.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req hostRequest
				if err := json.NewDecoder(c).Decode(&req); err != nil {
					return
				}
				result, ok := results[req.Method]
				resp := hostResponse{ID: req.ID, OK: ok, Result: result}
				if !ok {
					resp.Error = "method_not_allowed"
				}
				_ = json.NewEncoder(c).Encode(resp)
			}(conn)
		}
	}()

	return sock
}

func TestSnapshotAggregatesReadOnlyHostStatus(t *testing.T) {
	sock := startFakeHostd(t, map[string]map[string]any{
		"grol.system.status": {
			"status":       "ok",
			"grol_version": "18.4.dev",
		},
		"grol.network.status": {
			"status":       "ok",
			"connectivity": "online",
		},
		"grol.hardware.status": {
			"status":             "ok",
			"memory_pressure":    "ok",
			"memory_total_bytes": float64(1024),
		},
		"grol.update.status": {
			"status":      "ok",
			"active_slot": "A",
		},
		"grol.service.status": {
			"status": "ok",
			"components": map[string]any{
				"grol-hostd": "running",
				"supervisor": "running",
			},
		},
	})

	h := NewHealthd(sock)
	snapshot := h.Snapshot()

	if snapshot["ok"] != true || snapshot["status"] != "ok" {
		t.Fatalf("snapshot %#v", snapshot)
	}
	if snapshot["mutation_capable"] != false {
		t.Fatalf("observer must not be mutation capable: %#v", snapshot)
	}

	system, _ := snapshot["system"].(map[string]any)
	if system["grol_version"] != "18.4.dev" {
		t.Fatalf("system %#v", system)
	}
	network, _ := snapshot["network"].(map[string]any)
	if network["connectivity"] != "online" {
		t.Fatalf("network %#v", network)
	}
}

func TestSnapshotDegradesWhenHostdMissing(t *testing.T) {
	h := NewHealthd(filepath.Join(t.TempDir(), "missing.sock"))
	snapshot := h.Snapshot()

	if snapshot["ok"] != false ||
		snapshot["status"] != "degraded" ||
		snapshot["hostd_reachable"] != false {
		t.Fatalf("snapshot %#v", snapshot)
	}
}

func TestHealthdCannotCallMutationMethods(t *testing.T) {
	h := NewHealthd("/does/not/matter")
	_, err := h.hostCall("grol.host.reboot", nil)
	if err == nil || err.Error() != "method_not_allowed" {
		t.Fatalf("expected method_not_allowed, got %v", err)
	}
}
