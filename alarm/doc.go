// Package alarm sends deduplicated alert messages to a Slack incoming webhook.
//
// Two usage modes are supported:
//
//   - A package-level default instance, initialised once via Init and used
//     anywhere via the package-level Send function.
//   - Custom instances created with New, useful when multiple destinations or
//     dedup windows are needed.
//
// Messages are deduplicated within a configurable interval (default 5 minutes)
// to prevent the same alert from spamming the channel. The deduplication map
// is pruned automatically on each Send so memory usage stays bounded.
//
// If the webhook URL is empty, messages are written to stdout so the service
// keeps running in environments without alerting configured.
package alarm
