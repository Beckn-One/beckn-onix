package definition

import (
	"context"
	"time"
)

// Sync defines the interface for synchronous callback handling in Beckn protocol
// The plugin handles waiting for callbacks (BAP Caller) and publishing callbacks (BAP Receiver)
type Sync interface {
	Execute(ctx context.Context, txnID string, data []byte) (callbackData []byte, skipForwarding bool, err error)
}

type SyncProvider interface {
	New(ctx context.Context, cache Cache, role string, config map[string]string) (Sync, func() error, error)
}

type SyncConfig struct {
	Enabled       bool          // Enable/disable sync functionality (default: false)
	Timeout       time.Duration // Max wait time for callbacks (default: 30s)
	ChannelPrefix string        // Redis channel prefix (default: "sync:")
	KeyTTL        time.Duration // Redis key TTL (default: 35s)
}
