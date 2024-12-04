package utils

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.Mutex
	limit    int
	duration time.Duration
}

type Visitor struct {
	lastSeen time.Time
	tokens   int
}

// NewRateLimiter initializes the rate limiter
func NewRateLimiter(limit int, duration time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		limit:    limit,
		duration: duration,
	}
	go rl.cleanupVisitors()
	return rl
}

// AllowRequest checks and updates the request allowance for a specific IP
func (rl *RateLimiter) AllowRequest(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	visitor, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &Visitor{
			lastSeen: now,
			tokens:   rl.limit - 1,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(visitor.lastSeen)
	tokensToAdd := int(elapsed / rl.duration)
	if tokensToAdd > 0 {
		visitor.tokens = min(visitor.tokens+tokensToAdd, rl.limit)
		visitor.lastSeen = now
	}

	// Check if a request can be allowed
	if visitor.tokens > 0 {
		visitor.tokens--
		return true
	}

	return false
}

// Middleware for Gin
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Ignore OPTIONS preflight requests
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		if !rl.AllowRequest(clientIP) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests. Please try again later.",
			})
			return
		}
		c.Next()
	}
}

// cleanupVisitors periodically removes inactive visitors
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(rl.duration)
		rl.mu.Lock()
		for ip, visitor := range rl.visitors {
			if time.Since(visitor.lastSeen) > rl.duration {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}
