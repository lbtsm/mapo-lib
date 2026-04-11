package alarm

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

const (
	EnvWebhooks = "alarm_webhooks"
	EnvPrefix   = "alarm_prefix"
)

// Alarm sends deduplicated messages to a Slack incoming webhook.
// Messages with the same content are suppressed for a configurable interval.
type Alarm struct {
	prefix   string
	hookURL  string
	interval time.Duration

	mu   sync.RWMutex
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
// The local IP address is automatically appended to the prefix.
func New(prefix, hookURL string, opts ...Option) *Alarm {
	if hookURL == "" {
		hookURL = os.Getenv(EnvWebhooks)
	}
	if prefix == "" {
		prefix = os.Getenv(EnvPrefix)
	}
	if prefix == "" {
		prefix = fmt.Sprintf("unknown[%s]", localIP())
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

// defaultAlarm is the package-level default instance.
var defaultAlarm *Alarm

// Init initializes the default alarm instance. Call this once at startup.
func Init(prefix, hookURL string, opts ...Option) {
	defaultAlarm = New(prefix, hookURL, opts...)
}

// Send sends a message using the default alarm instance.
// Returns an error if Init has not been called.
func Send(ctx context.Context, msg string) error {
	if defaultAlarm == nil {
		return fmt.Errorf("alarm: default alarm not initialized, call Init first")
	}
	return defaultAlarm.Send(ctx, msg)
}

// Send posts a message to the configured Slack webhook.
// Duplicate messages within the deduplication interval are silently dropped.
// If the webhook URL is empty, the message is logged locally instead.
func (a *Alarm) Send(ctx context.Context, msg string) error {
	text := fmt.Sprintf("%s %s", a.prefix, msg)

	if a.hookURL == "" {
		fmt.Printf("[alarm] %s\n", text)
		return nil
	}

	now := time.Now().Unix()
	a.mu.RLock()
	last, ok := a.seen[msg]
	a.mu.RUnlock()
	if ok && now-last < int64(a.interval.Seconds()) {
		return nil
	}

	a.mu.Lock()
	a.seen[msg] = now
	a.mu.Unlock()

	return slack.PostWebhookContext(ctx, a.hookURL, &slack.WebhookMessage{Text: text})
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
