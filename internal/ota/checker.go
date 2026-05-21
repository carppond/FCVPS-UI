// Package ota implements the M-OPS OTA self-update subsystem (T-27).
//
// The subsystem has three orthogonal pieces:
//
//   - Checker  — polls the GitHub Release API for a newer tag than the running
//     binary (every 24h by default, also exposed as an on-demand admin call).
//   - Downloader — streams the binary asset into a sibling `.new` file while
//     emitting progress callbacks; verifies an attached SHA-256 hash before the
//     applier touches anything.
//   - Applier — checkpoints the SQLite WAL, atomically renames `<bin>.new`
//     into place (keeping a `.bak`) and triggers a graceful shutdown so the
//     external supervisor (systemd / docker restart=always) starts the new
//     process.
//
// Trust-root v1 (ADR 0008) is GitHub Release directly + SHA-256 verification.
// Sigstore/cosign keyless signing is a v1.5 follow-up; this package keeps the
// release pipeline as the only trust anchor.
package ota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultGitHubRepo is the upstream release source used when the
// OTA_GITHUB_REPO env-var is unset. Matches the v1 release pipeline; can be
// overridden at runtime to point at a private fork.
const DefaultGitHubRepo = "shiguang-vps/shiguang-vps"

// CheckInterval is the cadence of the background "is there a newer release?"
// sweep. Matches the documented `ota_check_interval` default (24h).
const CheckInterval = 24 * time.Hour

// DefaultGitHubAPIBase is the GitHub REST endpoint hit by Checker. Tests can
// swap this through CheckerConfig.APIBase to redirect at an httptest server.
const DefaultGitHubAPIBase = "https://api.github.com"

// ReleaseAsset describes a single artefact in a GitHub Release. We treat the
// `<asset>.sha256` sidecar as authoritative — the binary itself is downloaded
// only after its hash matches.
type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// ReleaseInfo aggregates the slim subset of GitHub's release JSON we depend on.
// `HasUpdate` is filled in by Checker.CheckLatest based on the running binary's
// version (it is not part of the upstream payload).
type ReleaseInfo struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`

	// HasUpdate is true when TagName resolves to a higher semver than the
	// running binary's version. Filled in by CheckLatest, not GitHub.
	HasUpdate bool `json:"-"`
	// CurrentVersion captures the running binary's version at the time of
	// the check; included so callers can render a clean diff in the UI.
	CurrentVersion string `json:"-"`
}

// CheckerConfig wires the Checker dependencies. All fields are optional;
// sensible defaults apply when zero values are supplied.
type CheckerConfig struct {
	// HTTPClient sends the GitHub API request. Defaults to a 15s-timeout
	// http.Client when nil. Tests substitute an httptest server's client here.
	HTTPClient *http.Client
	// GitHubRepo identifies the `owner/name` of the release source. Empty
	// falls back to DefaultGitHubRepo.
	GitHubRepo string
	// APIBase overrides the GitHub API host. Empty falls back to
	// DefaultGitHubAPIBase; tests use httptest.URL here.
	APIBase string
	// CurrentVersion is the running binary's semver (with or without the
	// leading "v"). When empty IsNewer treats any latest as newer so a dev
	// build still sees the update banner.
	CurrentVersion string
	// Now overrides time.Now; tests inject a fixed clock so PublishedAt
	// comparisons stay deterministic.
	Now func() time.Time
}

// Checker queries the GitHub Release API. It is goroutine-safe; the running
// binary may share a single Checker between the on-demand admin endpoint and
// the background sweep without locking.
type Checker struct {
	httpClient     *http.Client
	githubRepo     string
	apiBase        string
	currentVersion string
	now            func() time.Time
}

// NewChecker builds a Checker from cfg. Returns an error only when GitHubRepo
// is supplied but malformed (must contain exactly one slash).
func NewChecker(cfg CheckerConfig) (*Checker, error) {
	repo := cfg.GitHubRepo
	if repo == "" {
		repo = DefaultGitHubRepo
	}
	if strings.Count(repo, "/") != 1 || strings.HasPrefix(repo, "/") || strings.HasSuffix(repo, "/") {
		return nil, fmt.Errorf("ota: github repo must be \"owner/name\" (got %q)", repo)
	}
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = DefaultGitHubAPIBase
	}
	apiBase = strings.TrimRight(apiBase, "/")
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Checker{
		httpClient:     client,
		githubRepo:     repo,
		apiBase:        apiBase,
		currentVersion: cfg.CurrentVersion,
		now:            now,
	}, nil
}

// CheckLatest issues a single GET to `/repos/<repo>/releases/latest`. A 404
// (no release published yet) is surfaced as ErrNoRelease so the caller can
// distinguish "GitHub said nothing's there" from a transport failure.
func (c *Checker) CheckLatest(ctx context.Context) (*ReleaseInfo, error) {
	if c == nil {
		return nil, fmt.Errorf("ota: nil checker")
	}
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase, c.githubRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ota: build request: %w", err)
	}
	// GitHub recommends an explicit Accept header to lock the schema version.
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ota: github api: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// proceed
	case http.StatusNotFound:
		return nil, ErrNoRelease
	default:
		// Surface the status so logs have something actionable (rate-limit,
		// 401 from a bad token, transient 5xx, ...). We deliberately do not
		// retry here — the daily sweep will pick the next window up.
		return nil, fmt.Errorf("ota: github api status %d", resp.StatusCode)
	}
	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("ota: decode release: %w", err)
	}
	info.CurrentVersion = c.currentVersion
	info.HasUpdate = IsNewer(info.TagName, c.currentVersion)
	return &info, nil
}

// ErrNoRelease is returned by CheckLatest when GitHub answers 404 — typically a
// fresh repo with no published release. The daily watcher swallows it; the
// admin endpoint surfaces it as a friendlier "no release yet" message.
var ErrNoRelease = fmt.Errorf("ota: github has no release yet")

// IsNewer compares two semver strings (`vX.Y.Z[-suffix]` or `X.Y.Z`). The
// "v" prefix is stripped on both sides; pre-release suffixes are ignored for
// v1 simplicity (the release pipeline never ships -rc tags). Returns true
// when `latest` is strictly newer than `current`; false on parse failures so a
// malformed payload never triggers an upgrade.
//
// An empty `current` is treated as "0.0.0" so dev builds always see updates.
func IsNewer(latest, current string) bool {
	latestParts, ok := parseSemver(latest)
	if !ok {
		return false
	}
	currentParts, ok := parseSemver(current)
	if !ok {
		// Empty / malformed current → behave as 0.0.0 so the admin still sees
		// the upgrade banner. Defensive: missing version metadata in a dev
		// build should not silence OTA.
		currentParts = [3]int{0, 0, 0}
	}
	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

// parseSemver returns the (major, minor, patch) triple for v?X.Y.Z[-...]
// strings. Returns ok=false when the input cannot be parsed; pre-release
// suffixes (`-rc1`, `+build`) are stripped before parsing.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return [3]int{}, false
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	// Strip pre-release / build metadata: "1.2.3-rc1+abcd" → "1.2.3"
	if idx := strings.IndexAny(v, "-+"); idx > 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	out := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// PickAsset finds the asset matching `<binary>-<os>-<arch>` (or its `.sha256`
// sibling when suffix == ".sha256"). Returns the asset and true when found.
// Used by the applier to pair a binary URL with its hash URL in a single
// release payload.
func (r *ReleaseInfo) PickAsset(binaryName, os, arch, suffix string) (ReleaseAsset, bool) {
	if r == nil {
		return ReleaseAsset{}, false
	}
	target := fmt.Sprintf("%s-%s-%s%s", binaryName, os, arch, suffix)
	for _, a := range r.Assets {
		if a.Name == target {
			return a, true
		}
	}
	return ReleaseAsset{}, false
}
