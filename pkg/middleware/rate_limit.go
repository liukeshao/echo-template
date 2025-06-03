package middleware

import (
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/liukeshao/echo-template/pkg/errors"
)

// RateLimiter 限流器结构
type RateLimiter struct {
	mu          sync.RWMutex
	clients     map[string]*clientInfo
	rate        int           // 每分钟允许的请求数
	windowSize  time.Duration // 时间窗口大小
	cleanupTime time.Duration // 清理间隔
}

// clientInfo 客户端信息
type clientInfo struct {
	requests []time.Time
	lastSeen time.Time
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		clients:     make(map[string]*clientInfo),
		rate:        requestsPerMinute,
		windowSize:  time.Minute,
		cleanupTime: time.Minute * 5,
	}

	// 启动清理协程
	go rl.cleanup()

	return rl
}

// RateLimit 限流中间件
func (rl *RateLimiter) RateLimit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// 获取客户端IP
		clientIP := c.RealIP()
		if clientIP == "" {
			clientIP = c.Request().RemoteAddr
		}

		rl.mu.Lock()
		defer rl.mu.Unlock()

		now := time.Now()

		// 获取或创建客户端信息
		client, exists := rl.clients[clientIP]
		if !exists {
			client = &clientInfo{
				requests: make([]time.Time, 0),
				lastSeen: now,
			}
			rl.clients[clientIP] = client
		}

		// 更新最后访问时间
		client.lastSeen = now

		// 清理过期的请求记录
		cutoff := now.Add(-rl.windowSize)
		validRequests := make([]time.Time, 0)
		for _, requestTime := range client.requests {
			if requestTime.After(cutoff) {
				validRequests = append(validRequests, requestTime)
			}
		}
		client.requests = validRequests

		// 检查是否超过限制
		if len(client.requests) >= rl.rate {
			slog.WarnContext(ctx, "请求频率过高",
				"client_ip", clientIP,
				"requests_count", len(client.requests),
				"rate_limit", rl.rate,
			)

			// 设置限流相关的响应头
			c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
			c.Response().Header().Set("X-RateLimit-Remaining", "0")
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(rl.windowSize).Unix(), 10))

			return errors.New(429, "请求频率过高，请稍后重试")
		}

		// 记录本次请求
		client.requests = append(client.requests, now)

		// 设置响应头
		remaining := rl.rate - len(client.requests)
		c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.rate))
		c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(rl.windowSize).Unix(), 10))

		return next(c)
	}
}

// cleanup 清理过期的客户端记录
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupTime)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.cleanupTime)

		for clientIP, client := range rl.clients {
			if client.lastSeen.Before(cutoff) {
				delete(rl.clients, clientIP)
			}
		}
		rl.mu.Unlock()
	}
}

// GetStats 获取限流器统计信息
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"total_clients": len(rl.clients),
		"rate_limit":    rl.rate,
		"window_size":   rl.windowSize.String(),
	}
}
