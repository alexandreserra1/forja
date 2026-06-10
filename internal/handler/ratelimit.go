package handler

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rateLimiterEntry
	rps      rate.Limit
	burst    int
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(rps float64, burst int) *ipLimiter {
	l := &ipLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
	// Limpa entradas inativas a cada 5 minutos (sem vazamento de memória).
	go func() {
		for range time.Tick(5 * time.Minute) {
			l.mu.Lock()
			for ip, e := range l.limiters {
				if time.Since(e.lastSeen) > 10*time.Minute {
					delete(l.limiters, ip)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (il *ipLimiter) get(ip string) *rate.Limiter {
	il.mu.Lock()
	defer il.mu.Unlock()
	e, ok := il.limiters[ip]
	if !ok {
		e = &rateLimiterEntry{limiter: rate.NewLimiter(il.rps, il.burst)}
		il.limiters[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// RateLimit retorna um middleware que limita requisições por IP.
// rps = requisições por segundo permitidas; burst = pico instantâneo.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	il := newIPLimiter(rps, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !il.get(ip).Allow() {
				writeError(w, http.StatusTooManyRequests, fmt.Errorf("muitas requisições — tente novamente em instantes"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extrai o IP real do cliente, respeitando proxies via X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Pega o primeiro IP da lista (o do cliente real).
		if i := len(xff); i > 0 {
			for j := 0; j < i; j++ {
				if xff[j] == ',' {
					return xff[:j]
				}
			}
			return xff
		}
	}
	// RemoteAddr tem formato "ip:port".
	if host, _, err := splitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func splitHostPort(addr string) (host, port string, err error) {
	// Reimplementa net.SplitHostPort de forma leve para não importar "net" só por isso.
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "", fmt.Errorf("sem porta")
}
