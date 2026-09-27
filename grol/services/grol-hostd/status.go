package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var boot = time.Now()

var knownServices = []string{
	"grol-hostd", "grol-healthd", "grol-identity", "grol-provision",
	"grol-ai-gateway", "grol-bot", "grol-action-broker",
	"docker", "networkmanager", "rauc", "supervisor", "os-agent",
}

var serviceUnits = map[string]string{
	"grol-hostd":         "grol-hostd.service",
	"grol-healthd":       "grol-healthd.service",
	"grol-identity":      "grol-identity.service",
	"grol-provision":     "grol-provision.service",
	"grol-ai-gateway":    "grol-ai-gateway.service",
	"grol-bot":           "grol-bot.service",
	"grol-action-broker": "grol-action-broker.service",
	"docker":             "docker.service",
	"networkmanager":     "NetworkManager.service",
	"rauc":               "rauc.service",
	"supervisor":         "haos-supervisor.service",
	"os-agent":           "haos-agent.service",
}

func readKV(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return out
}

func cmdText(argv ...string) string {
	c := exec.Command(argv[0], argv[1:]...)
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func osRelease() map[string]string {
	m := readKV("/etc/os-release")
	if len(m) == 0 {
		m = readKV("/usr/lib/os-release")
	}
	return m
}

func systemStatus() map[string]any {
	osrel := osRelease()
	machine := readKV("/etc/machine-info")
	return map[string]any{
		"status":         "ok",
		"grol_version":   first(osrel["VERSION_ID"], "unknown"),
		"os_name":        first(osrel["NAME"], "GROL5000 OS"),
		"pretty_name":    first(osrel["PRETTY_NAME"], first(osrel["NAME"], "GROL5000 OS")),
		"board":          first(osrel["VARIANT_ID"], first(machine["CHASSIS"], "unknown")),
		"channel":        first(machine["DEPLOYMENT"], "unknown"),
		"uptime_seconds": int(time.Since(boot).Seconds()),
	}
}

func updateStatus() map[string]any {
	osrel := osRelease()
	rauc := cmdText("rauc", "status")
	slot := "unknown"
	raucState := "unavailable"
	if rauc != "" {
		raucState = "ok"
		for _, line := range strings.Split(rauc, "\n") {
			low := strings.ToLower(line)
			if strings.Contains(low, "booted") && strings.Contains(line, "[") {
				left := strings.TrimSpace(strings.Split(line, "[")[0])
				if len(left) > 0 {
					slot = strings.ToUpper(left[len(left)-1:])
				}
				break
			}
		}
	}
	st := "ok"
	if raucState == "unavailable" {
		st = "degraded"
	}
	return map[string]any{
		"status":             st,
		"os_version":         first(osrel["VERSION_ID"], "unknown"),
		"active_slot":        slot,
		"update_available":   false,
		"rauc":               raucState,
		"rollback_available": false,
		"components":         map[string]any{"rauc": raucState},
	}
}

func networkStatus() map[string]any {
	route := cmdText("ip", "-4", "route", "show", "default")
	hasDefault := route != ""
	var primary any
	if hasDefault {
		parts := strings.Fields(route)
		for i, p := range parts {
			if p == "dev" && i+1 < len(parts) {
				primary = parts[i+1]
				break
			}
		}
	}
	_, dnsErr := os.Stat("/etc/resolv.conf")
	dns := "unavailable"
	if dnsErr == nil {
		dns = "ok"
	}
	conn := "offline"
	st := "degraded"
	if hasDefault {
		conn = "online"
		st = "ok"
	}
	return map[string]any{
		"status":            st,
		"primary_interface": primary,
		"connectivity":      conn,
		"ipv4":              hasDefault,
		"default_route":     hasDefault,
		"dns":               dns,
	}
}

func hardwareStatus() map[string]any {
	var total, avail *int64
	f, err := os.Open("/proc/meminfo")
	if err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			n, convErr := strconv.ParseInt(fields[1], 10, 64)
			if convErr != nil {
				continue
			}
			n *= 1024
			switch {
			case strings.HasPrefix(line, "MemTotal:"):
				v := n
				total = &v
			case strings.HasPrefix(line, "MemAvailable:"):
				v := n
				avail = &v
			}
		}
		f.Close()
	}
	pressure := "unavailable"
	st := "degraded"
	if total != nil && avail != nil && *total > 0 {
		st = "ok"
		used := 1 - float64(*avail)/float64(*total)
		switch {
		case used > 0.9:
			pressure = "critical"
		case used > 0.75:
			pressure = "warn"
		default:
			pressure = "ok"
		}
	}
	return map[string]any{
		"status":                 st,
		"cpu_arch":               runtime.GOARCH,
		"memory_total_bytes":     total,
		"memory_available_bytes": avail,
		"memory_pressure":        pressure,
		"storage_pressure":       "unavailable",
	}
}

func unitState(unit string) string {
	text := cmdText("systemctl", "is-active", unit)
	switch text {
	case "active":
		return "running"
	case "inactive":
		return "stopped"
	case "activating", "deactivating", "reloading":
		return "degraded"
	default:
		return "unavailable"
	}
}

func serviceStatus(name string) map[string]any {
	if name != "" {
		unit, ok := serviceUnits[name]
		if !ok {
			return map[string]any{"status": "error", "error": "unknown_service", "service": name}
		}
		return map[string]any{"status": "ok", "service": name, "state": unitState(unit)}
	}
	components := map[string]any{}
	for _, item := range knownServices {
		components[item] = unitState(serviceUnits[item])
	}
	return map[string]any{"status": "ok", "components": components}
}

func first(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
