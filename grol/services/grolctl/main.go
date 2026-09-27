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
	brkURL := flag.String("broker", "http://127.0.0.1:8785", "grol-action-broker base URL")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: grolctl [--json] status|propose SERVICE ENTITY|grants list|grant add ENTITY SERVICE|proposal get ID|proposal confirm ID|audit recent")
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
	case args[0] == "propose":
		if len(args) < 3 {
			return map[string]any{"ok": false, "error": "usage"}, 2
		}
		return readResult(cli.Propose(args[1], args[2]))
	case verb == "grants" || verb == "grants list":
		return readResult(cli.Grants())
	case args[0] == "grant" && len(args) >= 4 && args[1] == "add":
		return readResult(cli.GrantAdd(args[2], args[3]))
	case args[0] == "grant" && len(args) >= 4 && args[1] == "revoke":
		return readResult(cli.GrantRevoke(args[2], args[3]))
	case args[0] == "proposal" && len(args) >= 3 && args[1] == "get":
		return readResult(cli.ProposalGet(args[2]))
	case args[0] == "proposal" && len(args) >= 3 && args[1] == "confirm":
		return readResult(cli.ProposalConfirm(args[2]))
	case verb == "audit" || verb == "audit recent":
		return readResult(cli.Audit())
	case args[0] == "raw-call", args[0] == "turn_on", args[0] == "call", args[0] == "apply":
		return map[string]any{"ok": false, "error": "mutation_disabled", "mutation_capable": false, "untrusted": true}, 3
	default:
		return map[string]any{"ok": false, "error": "unknown_verb", "verb": verb}, 2
	}
}

func renderText(out map[string]any) string {
	var b strings.Builder
	if ok, _ := out["ok"].(bool); !ok {
		fmt.Fprintf(&b, "error: %v\n", out["error"])
		return b.String()
	}
	if p, ok := out["proposal"].(map[string]any); ok {
		fmt.Fprintf(&b, "proposal %v\nstate: %v\ndecision: %v\nerror: %v\n", p["id"], p["state"], p["decision"], p["error"])
		return b.String()
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	b.Write(raw)
	b.WriteByte('\n')
	return b.String()
}
