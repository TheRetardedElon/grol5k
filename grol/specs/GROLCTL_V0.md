# grolctl v0

Narrow CLI used by Official Grok Bot on the operator PC.

Bot-approved verbs: status, system status, house snapshot, devices list, device get, activity recent, propose, proposal get, grants list, audit recent.

Operator-only verbs (require `--operator-token` matching broker `GROL_OPERATOR_TOKEN`):

- grant add / grant revoke
- proposal confirm

Confirm sends the frozen `confirm_digest` from the proposal. Apply stays disabled.

Grant storage currently keys by `entity_id + service` while also storing `registry_id`, `domain`, and `platform`. Stable-registry-ID lookup and rename following are **deferred** until haobs can prove live registry identity. Unproven grants are non-authorizing drafts.
