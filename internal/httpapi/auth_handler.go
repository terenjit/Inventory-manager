package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"inventory-manager/internal/auth"
)

// loginRateLimiter allows 5 attempts per IP per minute.
type loginRateLimiter struct {
	mu      sync.Mutex
	records map[string]*attemptRecord
}

type attemptRecord struct {
	count   int
	resetAt time.Time
}

func newLoginRateLimiter() *loginRateLimiter {
	return &loginRateLimiter{records: make(map[string]*attemptRecord)}
}

func (l *loginRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	rec, ok := l.records[ip]
	if !ok || now.After(rec.resetAt) {
		l.records[ip] = &attemptRecord{count: 1, resetAt: now.Add(time.Minute)}
		return true
	}
	if rec.count >= 5 {
		return false
	}
	rec.count++
	return true
}

func loginHandler(svc auth.Service, limiter *loginRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts, try again in a minute"})
			return
		}
		var input auth.LoginInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		token, err := svc.Login(c.Request.Context(), input)
		if err != nil {
			respondError(c, err)
			return
		}
		c.SetCookie("auth_token", token, int((12 * time.Hour).Seconds()), "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "login successful"})
	}
}

func logoutHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetCookie("auth_token", "", -1, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	}
}

func meHandler(svc auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		u, err := svc.Me(c.Request.Context(), actor.UserID)
		if err != nil {
			respondError(c, err)
			return
		}
		c.JSON(http.StatusOK, u)
	}
}
