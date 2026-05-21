package notify

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"
	"text/template"
)

//go:embed templates/*/*.tmpl
var templatesFS embed.FS

// DefaultLocale is the fallback when a recipient's locale is unset or no
// template file exists for it.
const DefaultLocale = "zh-CN"

// SupportedLocales lists the locales for which email templates ship in v1.
// Order matters: when the requested locale is missing, the renderer walks
// the list and falls back to the first match found.
var SupportedLocales = []string{"zh-CN", "en", "ja", "ko"}

// Template caches parsed templates per (locale, event_type, kind) tuple so
// the per-event Emit hot path does not re-parse from disk.
//
// templates/<locale>/<event_type>.<kind>.tmpl contains the renderable text.
// kind is one of "subject" / "body" / "html". When a specific (locale,
// event_type) tuple is missing, the renderer walks SupportedLocales until
// a match is found; if none match, it falls back to a generic "{{ .Subject
// }} — {{ .Type }} for resource {{ .ResourceID }}".
type Template struct {
	mu    sync.RWMutex
	cache map[string]*template.Template
}

// NewTemplate returns an empty template cache. Parsing is lazy.
func NewTemplate() *Template {
	return &Template{cache: make(map[string]*template.Template)}
}

// Render produces (subject, body) for the event using the locale-specific
// templates. When override is non-empty, it is used in place of the body
// template; subject still comes from the locale-specific subject file. The
// locale argument may be empty — DefaultLocale is then used.
func (t *Template) Render(event Event, override string) (subject, body string, err error) {
	locale := normaliseLocale(event.Locale)
	subj, err := t.renderOne(locale, string(event.Type), "subject", event)
	if err != nil {
		return "", "", fmt.Errorf("render subject: %w", err)
	}
	if subj == "" {
		// Fallback: use the caller-supplied Subject or a generic format.
		if event.Subject != "" {
			subj = event.Subject
		} else {
			subj = fmt.Sprintf("[shiguang-vps] %s", event.Type)
		}
	}
	var b string
	if override != "" {
		b, err = t.renderRaw(string(event.Type)+":override", override, event)
		if err != nil {
			return "", "", fmt.Errorf("render override: %w", err)
		}
	} else {
		b, err = t.renderOne(locale, string(event.Type), "body", event)
		if err != nil {
			return "", "", fmt.Errorf("render body: %w", err)
		}
	}
	if b == "" {
		b = fmt.Sprintf("Event %s — resource %s", event.Type, event.ResourceID)
	}
	return strings.TrimSpace(subj), strings.TrimSpace(b), nil
}

// RenderHTML returns the HTML-flavoured body (or the plain body when no .html
// template exists for the (locale, event_type) tuple). Used by the email
// channel which sends multipart/alternative.
func (t *Template) RenderHTML(event Event) (string, error) {
	locale := normaliseLocale(event.Locale)
	html, err := t.renderOne(locale, string(event.Type), "html", event)
	if err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	if html == "" {
		_, body, err := t.Render(event, "")
		if err != nil {
			return "", err
		}
		// Plain text → trivially escape to HTML. Newlines become <br>.
		escaped := strings.ReplaceAll(body, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		return "<p>" + escaped + "</p>", nil
	}
	return html, nil
}

// renderOne reads / parses templates/<locale>/<event>.<kind>.tmpl and
// renders it with event as the dot context. Falls back through
// SupportedLocales when the requested locale's file is missing. Returns ""
// when no locale has a matching file.
func (t *Template) renderOne(locale, eventType, kind string, event Event) (string, error) {
	tmpl, err := t.lookup(locale, eventType, kind)
	if err != nil {
		return "", err
	}
	if tmpl == nil {
		return "", nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, event); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// renderRaw parses an inline template string (used for per-channel custom
// templates). The result is NOT cached because each channel may carry a
// different override.
func (t *Template) renderRaw(name, raw string, event Event) (string, error) {
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse inline template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, event); err != nil {
		return "", fmt.Errorf("execute inline template: %w", err)
	}
	return buf.String(), nil
}

// lookup walks SupportedLocales until it finds an embedded file. nil is
// returned when no locale has a matching file (caller decides fallback).
func (t *Template) lookup(locale, eventType, kind string) (*template.Template, error) {
	for _, loc := range candidateLocales(locale) {
		cacheKey := loc + ":" + eventType + ":" + kind
		t.mu.RLock()
		if tmpl, ok := t.cache[cacheKey]; ok {
			t.mu.RUnlock()
			if tmpl == nil {
				// Negative cache entry — skip to next locale.
				continue
			}
			return tmpl, nil
		}
		t.mu.RUnlock()
		filename := fmt.Sprintf("%s.%s.tmpl", eventType, kind)
		data, err := fs.ReadFile(templatesFS, path.Join("templates", loc, filename))
		if err != nil {
			// Mark missing so we don't keep hitting embed.FS.
			t.mu.Lock()
			t.cache[cacheKey] = nil
			t.mu.Unlock()
			continue
		}
		tmpl, err := template.New(cacheKey).Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filename, err)
		}
		t.mu.Lock()
		t.cache[cacheKey] = tmpl
		t.mu.Unlock()
		return tmpl, nil
	}
	return nil, nil
}

// candidateLocales returns the lookup order — requested locale first, then
// every SupportedLocale in declared order (with duplicates filtered).
func candidateLocales(locale string) []string {
	out := make([]string, 0, len(SupportedLocales)+1)
	seen := make(map[string]bool, len(SupportedLocales)+1)
	if locale != "" {
		out = append(out, locale)
		seen[locale] = true
	}
	for _, l := range SupportedLocales {
		if !seen[l] {
			out = append(out, l)
			seen[l] = true
		}
	}
	return out
}

// normaliseLocale collapses missing / unsupported locales into DefaultLocale.
// We accept the bare language tag (e.g. "zh") and map to its primary region.
func normaliseLocale(locale string) string {
	switch locale {
	case "":
		return DefaultLocale
	case "zh":
		return "zh-CN"
	}
	return locale
}
