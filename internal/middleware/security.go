package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/time/rate"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	r        rate.Limit
	b        int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		r:        r,
		b:        b,
	}

	// Periodic cleanup of stale limiters
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			rl.mu.Lock()
			for ip, item := range rl.limiters {
				if time.Since(item.lastSeen) > 10*time.Minute {
					delete(rl.limiters, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return rl
}

func (rl *RateLimiter) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		ip := c.IP()

		rl.mu.Lock()
		item, exists := rl.limiters[ip]
		if !exists {
			item = &ipLimiter{
				limiter:  rate.NewLimiter(rl.r, rl.b),
				lastSeen: time.Now(),
			}
			rl.limiters[ip] = item
		} else {
			item.lastSeen = time.Now()
		}
		limiter := item.limiter
		rl.mu.Unlock()

		if !limiter.Allow() {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, please try again later",
			})
		}

		return c.Next()
	}
}

func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		return c.Next()
	}
}
