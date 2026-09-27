package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	jsonOut := flag.Bool("json", false, "machine-readable JSON")
	haURL := flag.String("ha-observer", "http://127.0.0.1:8786", "grol-haobs base URL")
	obsURL := flag.String("observer", "http://127.0.0.1:8787", "grol-healthd base URL")
	botURL := flag.String("bot", "http://127.0.0.1:8788", "grol-bot base URL")
	brkURL := flag.String("broker", "http://127.0.0.1:8785", "grol-action-broker base URL")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: grolctl [--json] status|system status|house snapshot|devices list|device get ID|activity recent|propose SERVICE ENTITY")
		os.Exit(2)
	}

	cli := NewClientWithBroker(*haURL, *obsURL, *botURL, *brkURL)
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

func run(cli *Client, args []string) (map[string]any, int) {
	verb := strings.Join(args, " ")
	switch {
	case verb == "status":
		return cli.Status(), 0
	case verb == "system" || verb == "system status":
		return withReadCode(cli.System())
	case verb == "house" || verb == "house snapshot":
		return withReadCode(cli.House())
	case verb == "devices" || verb == "devices list":
		return withReadCode(cli.Devices())
	case strings.HasPrefix(verb, "device get "):
		return withReadCode(cli.Device(strings.TrimSpace(strings.TrimPrefix(verb, "device get "))))
	case verb == "activity" || verb == "activity recent":
		return withReadCode(cli.Activity())
	case args[0] == "propose":
		if len(args) < 3 {
			return map[string]any{"ok": false, "error": "usage", "message": "grolctl propose SERVICE ENTITY_ID"}, 2
		}
		out := cli.Propose(args[1], args[2])
		if ok, _ := out["ok"].(bool); !ok {
			return out, 4
		}
		return out, 0
	case args[0] == "raw-call", args[0] == "turn_on", args[0] == "call", args[0] == "apply":
		return map[string]any{
			"ok": false, "error": "mutation_disabled", "mutation_capable": false, "untrusted": true,
			"message": "Use grolctl propose SERVICE ENTITY. Apply is broker-owned and disabled in M4.0.",
		}, 3
	default:
		return map[string]any{"ok": false, "error": "unknown_verb", "verb": verb}, 2
	}
}

func withReadCode(out map[string]any) (map[string]any, int) {
	if ok, _ := out["ok"].(bool); !ok {
		return out, 4
	}
	return out, 0
}

func renderText(out map[string]any) string {
	var b strings.Builder
	if ok, _ := out["ok"].(bool); !ok {
		fmt.Fprintf(&b, "error: %v\n", out["error"])
		if msg, _ := out["message"].(string); msg != "" {
			fmt.Fprintf(&b, "%s\n", msg)
		}
		return b.String()
	}
	if p, ok := out["proposal"].(map[string]any); ok {
		fmt.Fprintf(&b, "proposal %v\n%s %s\ndecision: %v\nerror: %v\n", p["id"], p["service"], p["entity_id"], p["decision"], p["error"])
		fmt.Fprintf(&b, "Read only apply: yes\nMutation capable: no\n")
		return b.String()
	}
	if c, ok := out["count"]; ok {
		fmt.Fprintf(&b, "%v entities visible\n", c)
	}
	if entity, ok := out["device"].(map[string]any); ok {
		fmt.Fprintf(&b, "%s\nstate: %v\n", entity["entity_id"], entity["state"])
	}
	fmt.Fprintf(&b, "Read only: yes\nMutation capable: no\n")
	return b.String()
}
