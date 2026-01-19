package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/helioLJ/rate_limiter_in_go/internal/config"
	"github.com/helioLJ/rate_limiter_in_go/internal/ratelimiter"
	ginmiddleware "github.com/helioLJ/rate_limiter_in_go/middleware/gin"
)

func main() {
	listenAddr := flag.String("listen", config.String("LISTEN_ADDR", ":8081"), "listen address")
	limit := flag.Int64("limit", config.Int64("RATE_LIMIT", 100), "requests per window")
	window := flag.Duration("window", config.Duration("WINDOW", time.Minute), "rate limit window")
	burst := flag.Int64("burst", config.Int64("BURST", 0), "burst capacity")
	redisAddr := flag.String("redis-addr", config.String("REDIS_ADDR", "localhost:6379"), "redis address")
	failMode := flag.String("fail-mode", config.String("FAIL_MODE", "open"), "open or closed")
	keyHeader := flag.String("key-header", config.String("KEY_HEADER", "X-API-Key"), "header used for rate limiting")
	flag.Parse()

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

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(ginmiddleware.GinMiddleware(limiter, nil, ginmiddleware.Options{
		KeyHeader: *keyHeader,
		Logger:    log.Default(),
	}))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello"})
	})

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on %s", *listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
