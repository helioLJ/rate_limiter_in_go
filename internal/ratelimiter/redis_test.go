package ratelimiter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisLimiterAllow(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set; skipping integration test")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	limiter, err := NewRedisLimiter(client, Options{
		Limit:  2,
		Window: time.Second,
		Burst:  0,
	})
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}

	key := fmt.Sprintf("test:%d", time.Now().UnixNano())

	for i := 0; i < 2; i++ {
		res, err := limiter.Allow(context.Background(), key)
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		if !res.Allowed {
			t.Fatalf("expected allowed at %d", i)
		}
	}

	res, err := limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("allow third: %v", err)
	}
	if res.Allowed {
		t.Fatalf("expected deny after limit")
	}

	time.Sleep(1200 * time.Millisecond)
	res, err = limiter.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("allow after sleep: %v", err)
	}
	if !res.Allowed {
		t.Fatalf("expected allowed after window reset")
	}
}
