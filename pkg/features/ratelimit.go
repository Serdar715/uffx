package features

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// AdaptiveLimiter implements an AIMD (Additive Increase, Multiplicative Decrease) rate limiter
// It now also handles WAF blocking logic, replacing the standalone WAFHook.
type AdaptiveLimiter struct {
	CurrentDelay time.Duration
	MinDelay     time.Duration
	MaxDelay     time.Duration
	mu           sync.Mutex
}

const (
	defaultMinDelay   = 10 * time.Millisecond
	defaultMaxDelay   = 30 * time.Second
	defaultStartDelay = 50 * time.Millisecond
	backoffMultiplier = 2
	recoveryRate      = 0.9
	wafBlockMinDelay  = 2 * time.Second
	wafErrorCode1     = 429
	wafErrorCode2     = 503
)

func NewAdaptiveLimiter() *AdaptiveLimiter {
	return &AdaptiveLimiter{
		CurrentDelay: defaultStartDelay,
		MinDelay:     defaultMinDelay,
		MaxDelay:     defaultMaxDelay,
	}
}

// Adjust updates the delay based on the response status code
// Returns the duration to sleep
func (limiter *AdaptiveLimiter) Adjust(statusCode int) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// WAF Detection
	if statusCode == wafErrorCode1 || statusCode == wafErrorCode2 {
		return limiter.handleBlock()
	}

	// Successful request — Additive Increase (Recovery)
	// Gradually reduce delay to recover throughput
	if limiter.CurrentDelay > limiter.MinDelay {
		limiter.CurrentDelay = time.Duration(float64(limiter.CurrentDelay) * recoveryRate)
		if limiter.CurrentDelay < limiter.MinDelay {
			limiter.CurrentDelay = limiter.MinDelay
		}
	}

	// Return current delay so the runner can apply consistent throttle during recovery
	return limiter.CurrentDelay
}

// BlockDetected explicitly signals a WAF block, forcing a backoff.
// This can be called by other hooks if they detect a block via other means (e.g. specific body content)
func (l *AdaptiveLimiter) BlockDetected() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.handleBlock()
}

// handleBlock calculates the backoff delay with Full Jitter (AWS best practice).
// Caller must hold the lock.
func (l *AdaptiveLimiter) handleBlock() time.Duration {
	// Multiplicative Decrease (Backoff)
	l.CurrentDelay *= backoffMultiplier

	// If delay is still small, jump to WAF pause minimum
	if l.CurrentDelay < wafBlockMinDelay {
		l.CurrentDelay = wafBlockMinDelay
	}

	if l.CurrentDelay > l.MaxDelay {
		l.CurrentDelay = l.MaxDelay
	}

	// Full Jitter: randomize between [MinDelay, CurrentDelay]
	// Prevents thundering herd when multiple goroutines hit WAF simultaneously
	jitterRange := int64(l.CurrentDelay - l.MinDelay)
	if jitterRange > 0 {
		return l.MinDelay + time.Duration(rand.Int63n(jitterRange))
	}
	return l.CurrentDelay
}

// GetDelay returns current penalty delay
func (l *AdaptiveLimiter) GetDelay() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.CurrentDelay
}

// CurrentRate calculates approxreq/sec based on delay (theoretical)
func (l *AdaptiveLimiter) CurrentRate() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.CurrentDelay == 0 {
		return 0
	}
	// 1 second / delay
	return math.Round(float64(time.Second) / float64(l.CurrentDelay))
}
