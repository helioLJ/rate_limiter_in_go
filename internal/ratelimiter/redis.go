package ratelimiter

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter enforces a token bucket using Redis + Lua for atomic updates.
type RedisLimiter struct {
	client   *redis.Client
	script   *redis.Script
	capacity int64
	rate     float64
	window   time.Duration
	ttl      time.Duration
	failOpen bool
}

func NewRedisLimiter(client *redis.Client, opts Options) (*RedisLimiter, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is nil: %w", ErrInvalidConfig)
	}
	if opts.Limit <= 0 || opts.Window <= 0 {
		return nil, fmt.Errorf("limit/window must be positive: %w", ErrInvalidConfig)
	}
	if opts.Burst < 0 {
		return nil, fmt.Errorf("burst must be >= 0: %w", ErrInvalidConfig)
	}

	capacity := opts.Limit + opts.Burst
	windowSeconds := opts.Window.Seconds()
	if windowSeconds <= 0 {
		return nil, fmt.Errorf("window must be >= 1s: %w", ErrInvalidConfig)
	}

	return &RedisLimiter{
		client:   client,
		script:   redis.NewScript(tokenBucketLua),
		capacity: capacity,
		rate:     float64(capacity) / windowSeconds,
		window:   opts.Window,
		ttl:      2 * opts.Window,
		failOpen: opts.FailOpen,
	}, nil
}

func (r *RedisLimiter) Allow(ctx context.Context, key string) (Result, error) {
	if key == "" {
		return Result{}, ErrEmptyKey
	}
	if ctx == nil {
		ctx = context.Background()
	}

	values, err := r.script.Run(ctx, r.client, []string{key}, r.capacity, r.rate, int64(r.ttl.Seconds())).Result()
	if err != nil {
		if r.failOpen {
			reset := time.Now().Add(r.window)
			return Result{
				Allowed:   true,
				Limit:     r.capacity,
				Remaining: r.capacity,
				Reset:     reset,
			}, nil
		}
		return Result{}, fmt.Errorf("redis script: %w", err)
	}

	res, err := parseLuaResult(values)
	if err != nil {
		return Result{}, err
	}

	if !res.Allowed {
		res.RetryAfter = time.Until(res.Reset)
	}

	return res, nil
}

func parseLuaResult(values interface{}) (Result, error) {
	arr, ok := values.([]interface{})
	if !ok || len(arr) < 4 {
		return Result{}, fmt.Errorf("unexpected lua result: %v", values)
	}

	allowed, err := toInt64(arr[0])
	if err != nil {
		return Result{}, err
	}
	remaining, err := toInt64(arr[1])
	if err != nil {
		return Result{}, err
	}
	resetUnix, err := toInt64(arr[2])
	if err != nil {
		return Result{}, err
	}
	limit, err := toInt64(arr[3])
	if err != nil {
		return Result{}, err
	}

	return Result{
		Allowed:   allowed == 1,
		Limit:     limit,
		Remaining: remaining,
		Reset:     time.Unix(resetUnix, 0),
	}, nil
}

func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
