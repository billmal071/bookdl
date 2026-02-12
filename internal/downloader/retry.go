package downloader

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/billmal071/bookdl/internal/config"
)

// RetryConfig holds retry settings
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns retry config from app settings
func DefaultRetryConfig() RetryConfig {
	cfg := config.Get()
	return RetryConfig{
		MaxAttempts: cfg.Network.RetryAttempts,
		BaseDelay:   cfg.Network.RetryBaseDelay,
		MaxDelay:    cfg.Network.RetryMaxDelay,
		Multiplier:  cfg.Network.RetryMultiplier,
	}
}

// ErrorCategory categorizes errors for retry decisions
type ErrorCategory int

const (
	// ErrorRetryable - temporary errors that should be retried
	ErrorRetryable ErrorCategory = iota
	// ErrorNonRetryable - permanent errors that should not be retried
	ErrorNonRetryable
	// ErrorRateLimited - rate limiting, should wait longer
	ErrorRateLimited
)

// CategorizeError determines how an error should be handled
func CategorizeError(err error, statusCode int) ErrorCategory {
	// Check status code first
	switch statusCode {
	case http.StatusTooManyRequests: // 429
		return ErrorRateLimited
	case http.StatusBadRequest, // 400
		http.StatusUnauthorized,        // 401
		http.StatusForbidden,           // 403
		http.StatusNotFound,            // 404
		http.StatusMethodNotAllowed,    // 405
		http.StatusGone,                // 410
		http.StatusRequestEntityTooLarge: // 413
		return ErrorNonRetryable
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,      // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return ErrorRetryable
	}

	// Check error types
	if err == nil {
		return ErrorRetryable
	}

	// Network errors are generally retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrorRetryable
		}
	}

	// Connection errors
	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"temporary failure",
		"timeout",
		"EOF",
		"broken pipe",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return ErrorRetryable
		}
	}

	// Default to non-retryable for unknown errors
	return ErrorNonRetryable
}

// CalculateBackoff calculates the next backoff duration with jitter
func CalculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	if attempt <= 0 {
		return cfg.BaseDelay
	}

	// Calculate exponential delay: base * multiplier^attempt
	delay := float64(cfg.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= cfg.Multiplier
	}

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add jitter (±25%)
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter

	return time.Duration(delay)
}

// RetryOperation executes an operation with exponential backoff
func RetryOperation(ctx context.Context, cfg RetryConfig, operation func() (int, error)) error {
	var lastErr error
	var statusCode int

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		statusCode, lastErr = operation()

		// Success
		if lastErr == nil {
			return nil
		}

		// Check if we should retry
		category := CategorizeError(lastErr, statusCode)

		switch category {
		case ErrorNonRetryable:
			return lastErr // Don't retry
		case ErrorRateLimited:
			// Wait longer for rate limiting (use max delay)
			if attempt < cfg.MaxAttempts-1 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(cfg.MaxDelay):
				}
			}
		case ErrorRetryable:
			// Normal exponential backoff
			if attempt < cfg.MaxAttempts-1 {
				backoff := CalculateBackoff(attempt, cfg)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
			}
		}
	}

	return lastErr
}
