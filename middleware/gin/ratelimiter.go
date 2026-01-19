package ginmiddleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/helioLJ/rate_limiter_in_go/internal/ratelimiter"
)

// KeyFunc resolves the rate-limiting key from the request.
type KeyFunc func(*gin.Context) (string, error)

// Options configure the Gin middleware behavior.
type Options struct {
	KeyHeader string
	Logger    *log.Logger
}

// GinMiddleware enforces rate limits for incoming Gin requests.
func GinMiddleware(limiter ratelimiter.Limiter, keyFunc KeyFunc, opts Options) gin.HandlerFunc {
	if keyFunc == nil {
		keyFunc = DefaultKeyFunc(opts.KeyHeader)
	}

	return func(c *gin.Context) {
		key, err := keyFunc(c)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid rate limit key")
			return
		}

		res, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.Printf("rate limiter error: %v", err)
			}
			respondError(c, http.StatusServiceUnavailable, "rate limiter unavailable")
			return
		}

		setRateLimitHeaders(c, res)

		if !res.Allowed {
			retryAfter := int64(res.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
			respondError(c, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		c.Next()
	}
}

// DefaultKeyFunc resolves a key using header, Authorization, or client IP.
func DefaultKeyFunc(header string) KeyFunc {
	return func(c *gin.Context) (string, error) {
		if header == "" {
			header = "X-API-Key"
		}
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value, nil
		}
		if auth := strings.TrimSpace(c.GetHeader("Authorization")); auth != "" {
			parts := strings.Fields(auth)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				return parts[1], nil
			}
			return auth, nil
		}

		if ip := c.ClientIP(); ip != "" {
			return ip, nil
		}

		return "", errors.New("missing key")
	}
}

func setRateLimitHeaders(c *gin.Context, res ratelimiter.Result) {
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", res.Reset.Unix()))
}

func respondError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message, "timestamp": time.Now().UTC().Format(time.RFC3339)})
}
