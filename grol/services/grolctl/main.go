package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

const readFailureExit = 4

func main() {
	jsonOut := flag.Bool("json", false, "machine-readable JSON")
	haURL := flag.String("ha-observer", "http://127.0.0.1:8786", "grol-haobs base URL")
	obsURL := flag.String("observer", "http://127.0.0.1:8787", "grol-healthd base URL")
	botURL := flag.String("bot", "http://127.0.0.1:8788", "grol-bot base URL")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: grolctl [--json] status|system status|house snapshot|devices list|device get ID|activity recent")
		os.Exit(2)
	}

	cli := NewClient(*haURL, *obsURL, *botURL)
	out, code := run(cli, args)
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Print(renderText(out))
	}
	os.Exit(code)
}

func readResult(out map[string]any) (map[string]any, int) {
	if ok, _ := out["ok"].(bool); !ok {
		return out, readFailureExit
	}
	return out, 0
}

func run(cli *Client, args []string) (map[string]any, int) {
	verb := strings.Join(args, " ")
	switch {
	case verb == "status":
		return cli.Status(), 0
	case verb == "system" || verb == "system status":
		return readResult(cli.System())
	case verb == "house" || verb == "house snapshot":
		return readResult(cli.House())
	case verb == "devices" || verb == "devices list":
		return readResult(cli.Devices())
	case strings.HasPrefix(verb, "device get "):
		return readResult(cli.Device(strings.TrimSpace(strings.TrimPrefix(verb, "device get "))))
	case verb == "activity" || verb == "activity recent":
		return readResult(cli.Activity())
	case strings.HasPrefix(args[0], "propose"), args[0] == "raw-call", args[0] == "turn_on", args[0] == "call":
		return map[string]any{
			"ok":               false,
			"error":            "mutation_disabled",
			"mutation_capable": false,
			"untrusted":        true,
			"message":          "M3D grolctl is read-only. Mutations go through the M4 broker as grolctl propose.",
		}, 3
	default:
		return map[string]any{
			"ok":      false,
			"error":   "unknown_verb",
			"verb":    verb,
			"message": "allowed: status, system status, house snapshot, devices list, device get ID, activity recent",
		}, 2
	}
}

func renderText(out map[string]any) string {
	var b strings.Builder
	if ok, _ := out["ok"].(bool); !ok {
		errStr, _ := out["error"].(string)
		status, _ := out["status"].(string)
		msg, _ := out["message"].(string)
		if errStr != "" {
			fmt.Fprintf(&b, "error: %s\n", errStr)
		}
		if status != "" {
			fmt.Fprintf(&b, "status: %s\n", status)
		}
		if msg != "" {
			fmt.Fprintf(&b, "%s\n", msg)
		}
		return b.String()
	}
	if c, ok := out["count"]; ok {
		fmt.Fprintf(&b, "%v entities visible\n", c)
	}
	if devices, ok := out["devices"].([]map[string]any); ok {
		for _, d := range devices {
			fmt.Fprintf(&b, "%-32s %s\n", d["entity_id"], d["state"])
		}
	}
	if entity, ok := out["device"].(map[string]any); ok {
		fmt.Fprintf(&b, "%s\nstate: %v\n", entity["entity_id"], entity["state"])
	}
	if _, ok := out["haobs"]; ok {
		fmt.Fprintf(&b, "haobs process:    %v\n", out["haobs"])
		fmt.Fprintf(&b, "house reachable: %v\n", out["house_reachable"])
		fmt.Fprintf(&b, "healthd process:  %v\n", out["healthd"])
		fmt.Fprintf(&b, "system reachable:%v\n", out["system_reachable"])
		fmt.Fprintf(&b, "grol-bot process: %v\n", out["bot"])
		if status, _ := out["status"].(string); status != "" {
			fmt.Fprintf(&b, "stack status:     %s\n", status)
		}
	}
	if snap, ok := out["snapshot"].(map[string]any); ok {
		raw, _ := json.MarshalIndent(snap, "", "  ")
		b.Write(raw)
		b.WriteByte('\n')
	}
	if acts, ok := out["activity"]; ok {
		raw, _ := json.MarshalIndent(acts, "", "  ")
		b.Write(raw)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "Read only: yes\nMutation capable: no\n")
	return b.String()
}
