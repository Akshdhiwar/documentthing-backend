package utils

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	visitors      map[string]*Visitor
	mu            sync.Mutex
	limit         int
	duration      time.Duration
	blockDuration time.Duration
}

type Visitor struct {
	requests   []time.Time
	blockUntil time.Time
}

// NewRateLimiter initializes the rate limiter
func NewRateLimiter(limit int, duration, blockDuration time.Duration) *RateLimiter {
	return &RateLimiter{
		visitors:      make(map[string]*Visitor),
		limit:         limit,
		duration:      duration,
		blockDuration: blockDuration,
	}
}

// AllowRequest checks and updates the request allowance for a specific IP
func (rl *RateLimiter) AllowRequest(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	visitor, exists := rl.visitors[ip]

	if !exists {
		rl.visitors[ip] = &Visitor{
			requests:   []time.Time{now},
			blockUntil: time.Time{},
		}
		return true
	}

	// Check if the user is currently blocked
	if now.Before(visitor.blockUntil) {
		return false
	}

	// Filter requests within the duration window
	validRequests := []time.Time{}
	for _, t := range visitor.requests {
		if now.Sub(t) <= rl.duration {
			validRequests = append(validRequests, t)
		}
	}
	visitor.requests = validRequests

	// Add the current request
	visitor.requests = append(visitor.requests, now)

	// Block user if they exceed the limit
	if len(visitor.requests) > rl.limit {
		visitor.blockUntil = now.Add(rl.blockDuration)
		log.Printf("IP %s is blocked until %s", ip, visitor.blockUntil.Format(time.RFC1123))
		return false
	}

	return true
}

// Middleware for Gin
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// Log incoming request
		log.Printf("Incoming request from IP: %s", clientIP)

		if !rl.AllowRequest(clientIP) {
			log.Printf("Request denied for IP: %s", clientIP)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message": "Too many requests. You are temporarily blocked. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
