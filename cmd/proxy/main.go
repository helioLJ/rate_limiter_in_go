package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/helioLJ/rate_limiter_in_go/internal/config"
	"github.com/helioLJ/rate_limiter_in_go/internal/ratelimiter"
)

func main() {
	listenAddr := flag.String("listen", config.String("PROXY_LISTEN_ADDR", ":8080"), "listen address")
	upstreamURL := flag.String("upstream", config.String("UPSTREAM_URL", "http://localhost:8081"), "upstream url")
	limit := flag.Int64("limit", config.Int64("RATE_LIMIT", 100), "requests per window")
	window := flag.Duration("window", config.Duration("WINDOW", time.Minute), "rate limit window")
	burst := flag.Int64("burst", config.Int64("BURST", 0), "burst capacity")
	redisAddr := flag.String("redis-addr", config.String("REDIS_ADDR", "localhost:6379"), "redis address")
	failMode := flag.String("fail-mode", config.String("FAIL_MODE", "open"), "open or closed")
	keyHeader := flag.String("key-header", config.String("KEY_HEADER", "X-API-Key"), "header used for rate limiting")
	flag.Parse()

	upstream, err := url.Parse(*upstreamURL)
	if err != nil {
		log.Fatalf("invalid upstream url: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: *redisAddr})
	defer func() { _ = client.Close() }()

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}

	limiter, err := ratelimiter.NewRedisLimiter(client, ratelimiter.Options{
		Limit:    *limit,
		Window:   *window,
		Burst:    *burst,
		FailOpen: strings.EqualFold(*failMode, "open"),
	})
	if err != nil {
		log.Fatalf("create limiter: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, proxyErr error) {
		log.Printf("proxy error: %v", proxyErr)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		key, err := resolveKey(r, *keyHeader)
		if err != nil {
			http.Error(w, "invalid rate limit key", http.StatusBadRequest)
			return
		}

		res, err := limiter.Allow(r.Context(), key)
		if err != nil {
			log.Printf("rate limiter error: %v", err)
			http.Error(w, "rate limiter unavailable", http.StatusServiceUnavailable)
			return
		}

		setRateLimitHeaders(w, res)

		if !res.Allowed {
			retryAfter := max(int64(res.RetryAfter.Seconds()), 1)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		proxy.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("proxy listening on %s", *listenAddr)
	log.Printf("proxy upstream %s", upstream.String())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func resolveKey(r *http.Request, header string) (string, error) {
	if header == "" {
		header = "X-API-Key"
	}
	if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
		return value, nil
	}
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		parts := strings.Fields(auth)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1], nil
		}
		return auth, nil
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host, nil
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr, nil
	}
	return "", fmt.Errorf("missing key")
}

func setRateLimitHeaders(w http.ResponseWriter, res ratelimiter.Result) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.Reset.Unix()))
}
