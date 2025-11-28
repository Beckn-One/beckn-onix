package definition

import (
	"context"
	"time"
)

// Cache defines the general cache interface for caching plugins.
type Cache interface {
	// Get retrieves a value from the cache based on the given key.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value in the cache with the given key and TTL (time-to-live) in seconds.
	Set(ctx context.Context, key, value string, ttl time.Duration) error

	// Delete removes a value from the cache based on the given key.
	Delete(ctx context.Context, key string) error

	// Clear removes all values from the cache.
	Clear(ctx context.Context) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// Subscribe subscribes to a Redis pub/sub channel.
	Subscribe(ctx context.Context, channel string) Subscription

	// Publish publishes a message to a Redis pub/sub channel.
	Publish(ctx context.Context, channel string, message []byte) error
}

// Subscription represents a Redis pub/sub subscription
type Subscription interface {
	// Channel returns the receive channel for messages
	Channel() <-chan *Message

	// Close closes the subscription
	Close() error
}

// Message represents a pub/sub message
type Message struct {
	Channel string
	Payload string
}

// CacheProvider interface defines the contract for managing cache instances.
type CacheProvider interface {
	// New initializes a new cache instance with the given configuration.
	New(ctx context.Context, config map[string]string) (Cache, func() error, error)
}
