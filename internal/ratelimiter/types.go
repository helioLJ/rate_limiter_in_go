package ratelimiter

import (
	"context"
	"time"
)

// Result represents a single rate limit decision.
type Result struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	Reset      time.Time
	RetryAfter time.Duration
}

// Limiter provides rate-limit decisions for a given key.
type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}

// Options configure the limiter behavior.
type Options struct {
	Limit    int64
	Window   time.Duration
	Burst    int64
	FailOpen bool
}
