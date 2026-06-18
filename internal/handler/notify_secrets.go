package handler

import "encoding/json"

// redactedSecret is the placeholder substituted for a channel's secret config
// values in API responses. It is a non-empty string so the web form's
// required-field validation still passes, and it round-trips back on save —
// the update path treats an incoming value equal to this sentinel as "keep the
// stored value" (see mergeChannelSecrets), so secrets never need to leave the
// server to be preserved across edits.
const redactedSecret = "********"

// channelSecretKeys maps a channel kind to the config keys whose values are
// sensitive (API tokens / keys / passwords / secret-bearing webhook URLs).
// These are never serialized to the client in cleartext.
var channelSecretKeys = map[string][]string{
	"telegram":   {"bot_token"},
	"discord":    {"webhook_url"},
	"slack":      {"webhook_url"},
	"email":      {"smtp_password"},
	"bark":       {"device_key"},
	"gotify":     {"app_token"},
	"webhook":    {"url", "headers"},
	"serverchan": {"send_key"},
	"pushdeer":   {"push_key"},
	"ifttt":      {"webhook_key"},
}

// redactChannelConfig returns a copy of cfg with the kind's secret values
// replaced by the sentinel. Non-secret keys (smtp_host, chat_id, …) are kept
// so the UI can still display them. Returns cfg unchanged when it isn't a map
// or the kind has no secrets.
func redactChannelConfig(kind string, cfg any) any {
	m, ok := cfg.(map[string]any)
	if !ok {
		return cfg
	}
	keys := channelSecretKeys[kind]
	if len(keys) == 0 {
		return cfg
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	for _, k := range keys {
		switch val := out[k].(type) {
		case string:
			if val != "" {
				out[k] = redactedSecret
			}
		case map[string]any: // e.g. webhook headers — redact each value
			hm := make(map[string]any, len(val))
			for hk, hv := range val {
				if s, ok := hv.(string); ok && s != "" {
					hm[hk] = redactedSecret
				} else {
					hm[hk] = hv
				}
			}
			out[k] = hm
		}
	}
	return out
}

// mergeChannelSecrets restores stored secret values into an incoming config map
// wherever the client sent back the sentinel (i.e. the user did not change that
// secret). Mutates incoming in place. Called on update BEFORE validation/save
// so the persisted config always holds real secrets.
func mergeChannelSecrets(kind string, incoming map[string]any, storedJSON string) {
	keys := channelSecretKeys[kind]
	if len(keys) == 0 || storedJSON == "" {
		return
	}
	var stored map[string]any
	if json.Unmarshal([]byte(storedJSON), &stored) != nil {
		return
	}
	for _, k := range keys {
		switch iv := incoming[k].(type) {
		case string:
			if iv == redactedSecret {
				if sv, ok := stored[k]; ok {
					incoming[k] = sv
				}
			}
		case map[string]any: // webhook headers
			sm, _ := stored[k].(map[string]any)
			for hk, hv := range iv {
				if s, ok := hv.(string); ok && s == redactedSecret && sm != nil {
					if sv, ok := sm[hk]; ok {
						iv[hk] = sv
					}
				}
			}
		}
	}
}
