package middleware

import (
	"huddle/pkg/response"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ClientRateData struct {
	StartTime time.Time
	Requests  int
}

type MemoryRateLimiter struct {
	mu           sync.Mutex
	clients      map[string]*ClientRateData
	maxRequests  int
	windowLength time.Duration
}

func NewMemoryRateLimiter(maxRequests int, windowLength time.Duration) *MemoryRateLimiter {
	limiter := &MemoryRateLimiter{
		clients:      make(map[string]*ClientRateData),
		maxRequests:  maxRequests,
		windowLength: windowLength,
	}

	// Periodic cleanup of expired client IP entries
	go func() {
		for {
			time.Sleep(2 * windowLength)
			limiter.mu.Lock()
			now := time.Now()
			for ip, data := range limiter.clients {
				if now.Sub(data.StartTime) > windowLength {
					delete(limiter.clients, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

var defaultLimiter = NewMemoryRateLimiter(60, 1*time.Minute)

func RateLimiter(c *gin.Context) {
	clientIP := c.ClientIP()

	defaultLimiter.mu.Lock()
	data, exists := defaultLimiter.clients[clientIP]
	now := time.Now()

	if !exists || now.Sub(data.StartTime) > defaultLimiter.windowLength {
		defaultLimiter.clients[clientIP] = &ClientRateData{
			StartTime: now,
			Requests:  1,
		}
		defaultLimiter.mu.Unlock()
		c.Next()
		return
	}

	data.Requests++
	if data.Requests > defaultLimiter.maxRequests {
		defaultLimiter.mu.Unlock()
		response.Error(c, http.StatusTooManyRequests, "Rate limit exceeded. Please try again later.", nil)
		c.Abort()
		return
	}

	defaultLimiter.mu.Unlock()
	c.Next()
}
