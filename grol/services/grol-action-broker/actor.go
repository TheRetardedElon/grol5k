package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
)

const confirmDigestVersion = "grol-confirm-v0"

func operatorConfigured() string {
	return strings.TrimSpace(os.Getenv("GROL_OPERATOR_TOKEN"))
}

func isOperator(r *http.Request) bool {
	if r.Header.Get("X-GROL-Actor") != "operator" {
		return false
	}
	want := operatorConfigured()
	got := r.Header.Get("X-GROL-Operator-Token")
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func writeOperatorDenied(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{
		"ok": false, "error": "operator_required",
		"message": "grant and confirm are operator-only. Bot context cannot create grants or confirm proposals.",
	})
}

func canonicalJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(raw)
}

func confirmDigest(tool string, args any, targets []map[string]string, risk string) string {
	sorted := append([]map[string]string(nil), targets...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i]["registry_id"]+sorted[i]["entity_id"] < sorted[j]["registry_id"]+sorted[j]["entity_id"]
	})
	pre := confirmDigestVersion + "\x00" + tool + "\x00" + canonicalJSON(args) + "\x00" + canonicalJSON(sorted) + "\x00" + risk
	sum := sha256.Sum256([]byte(pre))
	return hex.EncodeToString(sum[:])
}

func deviceIdentity(dev map[string]any) (registryID, domain, platform string, proven bool) {
	registryID, _ = dev["registry_id"].(string)
	domain, _ = dev["domain"].(string)
	platform, _ = dev["platform"].(string)
	flag, _ := dev["identity_proven"].(bool)
	proven = flag && registryID != "" && domain != "" && platform != ""
	return
}
