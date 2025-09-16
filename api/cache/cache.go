package cache

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
)

// Config holds cache configuration
type Config struct {
	// Expiration time for the cache
	Expiration time.Duration
	// IncludeQueryParams determines if query parameters should be included in cache key
	IncludeQueryParams bool
}

// DefaultConfig returns a default cache configuration
func DefaultConfig() Config {
	return Config{
		Expiration:         time.Second,
		IncludeQueryParams: true,
	}
}

// New creates a new cache middleware with the given configuration
func New(cfg Config) fiber.Handler {
	cacheConfig := cache.Config{
		Expiration: cfg.Expiration,
	}

	// If query parameters should be included, use custom KeyGenerator
	if cfg.IncludeQueryParams {
		cacheConfig.KeyGenerator = func(c *fiber.Ctx) string {
			// Include method, path and query string in cache key
			// Method is important to differentiate GET, POST, etc.
			queryString := string(c.Request().URI().QueryString())
			if queryString != "" {
				return c.Method() + ":" + c.Path() + "?" + queryString
			}
			return c.Method() + ":" + c.Path()
		}
	}

	return cache.New(cacheConfig)
}

// WithExpiration creates a cache middleware with custom expiration time
func WithExpiration(expiration time.Duration) fiber.Handler {
	cfg := DefaultConfig()
	cfg.Expiration = expiration
	return New(cfg)
}
