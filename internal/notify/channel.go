package notify

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Message is the rendered payload handed to a Channel's Send. Subject is the
// short title (used as email subject / push notification title); Body is the
// long-form text. Locale is the recipient's preferred language and is
// surfaced for channels (e.g. Bark, Telegram parse_mode) that need it.
type Message struct {
	EventType string
	Subject   string
	Body      string
	Locale    string
}

// Channel is the contract every concrete notifier implements. Kind returns
// the unique string identifier (matches types.ChannelKind). Send delivers the
// rendered Message via the channel's transport; Validate runs sanity checks
// on a raw config blob before persisting.
//
// Implementations are expected to be lightweight (one struct field per
// transport endpoint). The Manager constructs them via factories registered
// at startup.
type Channel interface {
	Kind() string
	Send(ctx context.Context, cfg any, msg Message) error
	Validate(cfg any) error
}

// Factory builds a Channel instance from a generic config map. The map shape
// matches the kind's Config struct (see internal/types/api.go §M-NOTIFY).
type Factory func(cfg map[string]any) (Channel, error)

// Errors surfaced by the registry.
var (
	// ErrUnknownChannelKind is returned by Registry.Build when the requested
	// kind has not been registered.
	ErrUnknownChannelKind = errors.New("notify: unknown channel kind")
)

// Registry is the thread-safe lookup table that maps a channel kind string
// to its Factory. main() seeds it with the 5 batch-1 implementations; the
// Manager consults it when building a Channel from a stored config_json.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry returns an empty registry. Use the package-level Default for
// the production seed.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register installs a factory for the given kind. Subsequent Register calls
// overwrite the previous binding so test setup can swap mocks. Concurrent
// callers are serialised under a write lock.
func (r *Registry) Register(kind string, f Factory) {
	if r == nil || kind == "" || f == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[kind] = f
}

// Build constructs a Channel from the stored config map. ErrUnknownChannelKind
// is returned when no factory is registered for kind.
func (r *Registry) Build(kind string, cfg map[string]any) (Channel, error) {
	if r == nil {
		return nil, ErrUnknownChannelKind
	}
	r.mu.RLock()
	f, ok := r.factories[kind]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownChannelKind, kind)
	}
	ch, err := f(cfg)
	if err != nil {
		return nil, fmt.Errorf("build channel %q: %w", kind, err)
	}
	return ch, nil
}

// List returns the registered kinds in lexicographic order. Used by the
// admin UI's "available channel types" picker (T-25).
func (r *Registry) List() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	out := make([]string, 0, len(r.factories))
	for kind := range r.factories {
		out = append(out, kind)
	}
	r.mu.RUnlock()
	sort.Strings(out)
	return out
}

// DefaultRegistry is the process-wide registry seeded by RegisterBuiltins.
// Tests should use NewRegistry to avoid cross-test pollution.
var DefaultRegistry = NewRegistry()

// RegisterBuiltins seeds reg with all built-in channels: the five batch-1
// channels (telegram, discord, slack, email, bark) and the five batch-2
// channels (gotify, webhook, serverchan, pushdeer, ifttt).
// The function is idempotent (Register simply overwrites).
func RegisterBuiltins(reg *Registry) {
	if reg == nil {
		return
	}
	// batch 1
	reg.Register("telegram", buildTelegram)
	reg.Register("discord", buildDiscord)
	reg.Register("slack", buildSlack)
	reg.Register("email", buildEmail)
	reg.Register("bark", buildBark)
	// batch 2
	reg.Register("gotify", buildGotify)
	reg.Register("webhook", buildWebhook)
	reg.Register("serverchan", buildServerChan)
	reg.Register("pushdeer", buildPushDeer)
	reg.Register("ifttt", buildIFTTT)
}
