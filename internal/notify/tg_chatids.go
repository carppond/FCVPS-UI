package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// channelConfig is the in-memory projection of a telegram channel config
// blob. The single-chat form (chat_id: string) is the legacy shape used by
// T-22 (one chat per channel); T-24 introduces chat_ids: []string so a user
// can /start the bot from multiple devices. Both representations are kept
// alive for backwards compatibility.
type channelConfig map[string]any

// decodeChannelConfig parses the on-disk JSON into a mutable map. An empty
// input is treated as an empty config (returns map[]).
func decodeChannelConfig(raw string) (channelConfig, error) {
	if raw == "" {
		return channelConfig{}, nil
	}
	var out channelConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode telegram config: %w", err)
	}
	if out == nil {
		out = channelConfig{}
	}
	return out, nil
}

// encodeChannelConfig serialises the mutable map back to JSON.
func encodeChannelConfig(cfg channelConfig) (string, error) {
	if cfg == nil {
		return "{}", nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encode telegram config: %w", err)
	}
	return string(b), nil
}

// appendChatID adds chatID to cfg["chat_ids"], folding in the legacy
// cfg["chat_id"] field. Returns true when the chat was newly added; false
// when it was already present.
func appendChatID(cfg channelConfig, chatID int64) bool {
	if cfg == nil {
		return false
	}
	chatStr := strconv.FormatInt(chatID, 10)
	existing := readChatIDs(cfg)
	for _, c := range existing {
		if c == chatStr {
			return false
		}
	}
	existing = append(existing, chatStr)
	cfg["chat_ids"] = existing
	// Mirror first entry into the legacy chat_id field so ch_telegram.go
	// (single-chat sender) keeps working unchanged.
	if _, ok := cfg["chat_id"]; !ok {
		cfg["chat_id"] = existing[0]
	}
	return true
}

// readChatIDs returns the cfg["chat_ids"] list, folding in cfg["chat_id"]
// when the array form is absent.
func readChatIDs(cfg channelConfig) []string {
	if cfg == nil {
		return nil
	}
	if raw, ok := cfg["chat_ids"]; ok {
		switch v := raw.(type) {
		case []any:
			out := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			return out
		case []string:
			return append([]string(nil), v...)
		}
	}
	if raw, ok := cfg["chat_id"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			return []string{s}
		}
	}
	return nil
}

// chatIDsMatch reports whether chatID is present in any of the
// telegram-kind channel configs in recs. Returns the owning user_id of the
// first match. Used by the whitelist resolver to route inbound updates.
func chatIDsMatch(recs []chatBinding, chatID int64) (userID, locale string, ok bool) {
	target := chatID
	for _, b := range recs {
		for _, cid := range b.ChatIDs {
			parsed, err := strconv.ParseInt(cid, 10, 64)
			if err != nil {
				continue
			}
			if parsed == target {
				return b.UserID, b.Locale, true
			}
		}
	}
	return "", "", false
}

// chatBinding is the projection the whitelist resolver builds at refresh
// time. Each entry pairs a user with the chat IDs registered on every one
// of their telegram channels.
type chatBinding struct {
	UserID  string
	Locale  string
	ChatIDs []string
}

// ErrWhitelistEmpty signals "no telegram channels configured" — the bot
// silently drops every update in this state.
var ErrWhitelistEmpty = errors.New("notify: no telegram channels configured")
