package alarm

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slack-go/slack"
)

const (
	EnvWebhooks = "alarm_webhooks"
	EnvPrefix   = "alarm_prefix"
)

// Sender is the contract implemented by *Alarm. Consumers can depend on this
// interface in their own code so they can substitute a fake in tests.
type Sender interface {
	Send(ctx context.Context, msg string) error
}

// Alarm sends deduplicated messages to a Slack incoming webhook.
// Messages with the same content are suppressed for a configurable interval.
type Alarm struct {
	prefix   string
	hookURL  string
	interval time.Duration

	mu   sync.Mutex
	seen map[string]int64
}

// Option configures an Alarm instance.
type Option func(*Alarm)

// WithInterval sets the deduplication window. Default is 5 minutes.
func WithInterval(d time.Duration) Option {
	return func(a *Alarm) { a.interval = d }
}

// New creates an Alarm with the given environment prefix and Slack webhook URL.
// If hookURL is empty, it falls back to the alarm_webhooks environment variable.
// If prefix is empty, it falls back to alarm_prefix env, then "unknown".
// The local IP address is appended to the prefix lazily on first use, so New
// itself performs no network I/O.
func New(prefix, hookURL string, opts ...Option) *Alarm {
	if hookURL == "" {
		hookURL = os.Getenv(EnvWebhooks)
	}
	if prefix == "" {
		prefix = os.Getenv(EnvPrefix)
	}

	a := &Alarm{
		prefix:   prefix,
		hookURL:  hookURL,
		interval: 5 * time.Minute,
		seen:     make(map[string]int64),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// defaultAlarm is the package-level default instance, swapped atomically so
// concurrent Init / Send calls don't race.
var defaultAlarm atomic.Pointer[Alarm]

// Init initializes the default alarm instance. Safe to call concurrently with
// Send; the swap is atomic.
func Init(prefix, hookURL string, opts ...Option) {
	defaultAlarm.Store(New(prefix, hookURL, opts...))
}

// Send sends a message using the default alarm instance.
// Returns an error if Init has not been called.
func Send(ctx context.Context, msg string) error {
	a := defaultAlarm.Load()
	if a == nil {
		return fmt.Errorf("alarm: default alarm not initialized, call Init first")
	}
	return a.Send(ctx, msg)
}

// Send posts a message to the configured Slack webhook.
// Duplicate messages within the deduplication interval are silently dropped.
// If the webhook URL is empty, the message is logged locally instead.
func (a *Alarm) Send(ctx context.Context, msg string) error {
	text := fmt.Sprintf("%s %s", a.resolvedPrefix(), msg)

	if a.hookURL == "" {
		fmt.Printf("[alarm] %s\n", text)
		return nil
	}

	now := time.Now().UnixNano()
	intervalNs := a.interval.Nanoseconds()

	// Single critical section: prune stale entries, check the dedup window,
	// and reserve the slot for this message — all atomically. Reserving
	// before the network call ensures that concurrent goroutines sending the
	// same message see the entry and skip, so only one HTTP request goes out.
	a.mu.Lock()
	// Prune entries older than 2*interval. n is bounded by recent unique
	// messages so this is fine.
	cutoff := now - 2*intervalNs
	for k, ts := range a.seen {
		if ts < cutoff {
			delete(a.seen, k)
		}
	}
	if last, ok := a.seen[msg]; ok && now-last < intervalNs {
		a.mu.Unlock()
		return nil
	}
	a.seen[msg] = now
	a.mu.Unlock()

	if err := slack.PostWebhookContext(ctx, a.hookURL, &slack.WebhookMessage{Text: text}); err != nil {
		// Remove the reservation so the next attempt isn't suppressed by a
		// failure that never reached anyone.
		a.mu.Lock()
		// Only delete if no later success has overwritten our timestamp.
		if a.seen[msg] == now {
			delete(a.seen, msg)
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

// seenSize returns the number of entries currently held in the dedup map.
// It is unexported and intended for tests.
func (a *Alarm) seenSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.seen)
}

// resolvedPrefix returns the configured prefix, lazily appending the local IP
// the first time it is needed. This keeps New() free of network I/O.
func (a *Alarm) resolvedPrefix() string {
	if a.prefix != "" {
		return a.prefix
	}
	return fmt.Sprintf("unknown[%s]", cachedLocalIP())
}

var (
	localIPOnce  sync.Once
	localIPValue string
)

// cachedLocalIP resolves the preferred outbound IP once and caches the result.
func cachedLocalIP() string {
	localIPOnce.Do(func() {
		localIPValue = localIP()
	})
	return localIPValue
}

// localIP returns the preferred outbound IP of this machine.
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}
