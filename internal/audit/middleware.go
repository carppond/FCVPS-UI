package audit

import (
	"net/http"
	"strings"

	"shiguang-vps/internal/handler/middleware"
)

// ResourceExtractor classifies a request into (resource_type, resource_id).
// Returned strings are stored as-is in audit_logs columns of the same name.
type ResourceExtractor func(r *http.Request) (resourceType, resourceID string)

// ActionExtractor classifies a request into a business-action string
// ("create_subscription", "rotate_share_token", …). When the extractor
// returns the empty string the middleware falls back to "<METHOD> <path>"
// — useful while the action map is being filled in.
type ActionExtractor func(r *http.Request) string

// PathActionMap maps a (method, path-prefix) pair to the canonical audit
// action name. The middleware iterates this map in declaration order so
// longest-prefix-wins is the caller's responsibility — order from most
// specific to least specific.
//
// Per §2.8 the 14 canonical actions are:
//
//	login / logout
//	create_subscription / delete_subscription / sync_subscription
//	create_pipeline / delete_user
//	ota_apply / silent_mode_rotate / rotate_share_token / rotate_agent_token
//	enable_2fa / disable_2fa
//	(plus admin's audit-read which the audit handler logs itself)
//
// The list is intentionally hard-coded here — adding a new action requires
// touching this file so the central catalog stays in sync.
var defaultActionMap = []actionRule{
	{Method: "POST", Path: "/api/auth/login", Action: "login"},
	{Method: "POST", Path: "/api/auth/logout", Action: "logout"},
	{Method: "POST", Path: "/api/auth/verify-totp", Action: "verify_totp"},
	{Method: "POST", Path: "/api/auth/verify-recovery", Action: "verify_recovery"},

	{Method: "POST", Path: "/api/me/totp/enable", Action: "enable_2fa"},
	{Method: "POST", Path: "/api/me/totp/disable", Action: "disable_2fa"},
	{Method: "POST", Path: "/api/me/totp/recovery-codes", Action: "regen_recovery_codes"},
	{Method: "DELETE", Path: "/api/me", Action: "delete_self"},

	{Method: "POST", Path: "/api/admin/users/", Suffix: "/disable-2fa", Action: "admin_disable_2fa"},
	{Method: "POST", Path: "/api/admin/users/", Suffix: "/reset-password", Action: "admin_reset_password"},
	{Method: "DELETE", Path: "/api/admin/users/", Action: "delete_user"},
	{Method: "POST", Path: "/api/admin/users", Action: "create_user"},

	{Method: "POST", Path: "/api/subscriptions/", Suffix: "/sync", Action: "sync_subscription"},
	{Method: "POST", Path: "/api/subscriptions/", Suffix: "/rotate-share-token", Action: "rotate_share_token"},
	{Method: "POST", Path: "/api/subscriptions", Action: "create_subscription"},
	{Method: "DELETE", Path: "/api/subscriptions/", Action: "delete_subscription"},

	{Method: "POST", Path: "/api/pipelines", Action: "create_pipeline"},
	{Method: "DELETE", Path: "/api/pipelines/", Action: "delete_pipeline"},

	{Method: "POST", Path: "/api/agents/", Suffix: "/rotate-token", Action: "rotate_agent_token"},
	{Method: "POST", Path: "/api/agents/", Suffix: "/regen-token", Action: "rotate_agent_token"},
	{Method: "POST", Path: "/api/agents", Action: "create_agent"},
	{Method: "DELETE", Path: "/api/agents/", Action: "delete_agent"},

	{Method: "POST", Path: "/api/admin/ota/apply", Action: "ota_apply"},

	{Method: "POST", Path: "/api/admin/silent-mode/rotate", Action: "silent_mode_rotate"},
	{Method: "POST", Path: "/api/admin/settings/silent-mode", Action: "silent_mode_rotate"},
	{Method: "PUT", Path: "/api/admin/settings", Action: "update_settings"},
	{Method: "PATCH", Path: "/api/admin/settings", Action: "update_settings"},

	{Method: "POST", Path: "/api/admin/backup", Action: "backup_create"},
	{Method: "POST", Path: "/api/admin/backup/restore", Action: "backup_restore"},
	{Method: "POST", Path: "/api/admin/restore", Action: "backup_restore"},

	{Method: "POST", Path: "/api/shortlinks", Action: "create_shortlink"},
	{Method: "DELETE", Path: "/api/shortlinks/", Action: "delete_shortlink"},
}

// actionRule is a single (method, path-prefix [, suffix]) → action mapping.
type actionRule struct {
	Method string
	Path   string
	Suffix string
	Action string
}

// ClassifyAction returns the canonical action name for r, or "" when no
// rule matches. The middleware uses this to populate AuditEntry.Action;
// rules with both Path and Suffix require both to match.
func ClassifyAction(r *http.Request) string {
	method := strings.ToUpper(r.Method)
	path := r.URL.Path
	for _, rule := range defaultActionMap {
		if rule.Method != method {
			continue
		}
		if rule.Path == path {
			return rule.Action
		}
		if !strings.HasSuffix(rule.Path, "/") && rule.Suffix == "" {
			continue
		}
		if !strings.HasPrefix(path, rule.Path) {
			continue
		}
		if rule.Suffix != "" && !strings.HasSuffix(path, rule.Suffix) {
			continue
		}
		return rule.Action
	}
	return ""
}

// ExtractResource classifies the URL into (resource_type, resource_id) using
// a coarser heuristic than the generic default in middleware/audit.go:
// it knows about the canonical /api/<plural>/{id} layout the project uses
// universally.
func ExtractResource(r *http.Request) (string, string) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for len(parts) > 0 && (parts[0] == "api" || parts[0] == "admin" || parts[0] == "v1") {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "", ""
	}
	resource := parts[0]
	resourceID := ""
	if len(parts) >= 2 {
		// Heuristic: anything that contains a hyphen or is ≥ 8 chars long
		// looks like an ID. Verb-like segments (sync, rotate, …) are filtered
		// out by the canonical paths registered in the router.
		candidate := parts[1]
		if len(candidate) >= 4 && !isReservedVerb(candidate) {
			resourceID = candidate
		}
	}
	return resource, resourceID
}

// isReservedVerb returns true for the trailing path segments the audit
// classifier should NOT treat as IDs. Mirrors the controller verbs the
// router registers.
var reservedVerbs = map[string]struct{}{
	"sync":                 {},
	"upload":               {},
	"reset-password":       {},
	"disable-2fa":          {},
	"revoke-sessions":      {},
	"rotate-share-token":   {},
	"rotate-token":         {},
	"regen-token":          {},
	"copy-uri":             {},
	"run":                  {},
	"yaml-to-ast":          {},
	"ast-to-yaml":          {},
	"operators":            {},
	"templates":            {},
	"reorder":              {},
	"preview":              {},
	"test":                 {},
	"logs":                 {},
	"single":               {},
	"batch":                {},
	"refresh":              {},
	"check":                {},
	"apply":                {},
	"status":               {},
	"history":              {},
	"summary":              {},
	"by-agent":             {},
	"threshold":            {},
	"limit":                {},
	"command":              {},
	"restart":              {},
	"records":              {},
	"backup":                {},
	"restore":              {},
	"setup":                {},
	"enable":               {},
	"disable":              {},
	"recovery-codes":       {},
	"sessions":             {},
	"channels":             {},
	"events":               {},
	"stream":               {},
	"silent-mode":          {},
	"settings":             {},
	"rotate":               {},
	"shortlinks":           {},
}

func isReservedVerb(s string) bool {
	_, ok := reservedVerbs[s]
	return ok
}

// BuildConfig returns an AuditConfig wiring the middleware to the supplied
// repo plus the project's action / resource extractors. Pass the resulting
// config to middleware.Audit.
func BuildConfig(repo middleware.AuditRepository) middleware.AuditConfig {
	return middleware.AuditConfig{
		Repo:            repo,
		ExtractResource: ExtractResource,
	}
}
