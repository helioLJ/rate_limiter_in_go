package ratelimiter

import "errors"

var (
	ErrEmptyKey      = errors.New("empty rate limit key")
	ErrInvalidConfig = errors.New("invalid rate limiter configuration")
)
