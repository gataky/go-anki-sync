package anki

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// retryWithBackoff retries a function with exponential backoff for transient failures.
// It returns nil on success or the last error after all retries are exhausted.
func retryWithBackoff(operation string, maxAttempts int, fn func() error) error {
	baseDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn()

		if err == nil {
			return nil // Success!
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return err // Fail immediately for non-retryable errors
		}

		// Last attempt - don't sleep, just return error
		if attempt == maxAttempts {
			return fmt.Errorf("%s failed after %d attempts: %w", operation, maxAttempts, lastErr)
		}

		// Calculate delay with exponential backoff
		delay := baseDelay * time.Duration(1<<(attempt-1))
		if delay > maxDelay {
			delay = maxDelay
		}

		// Add jitter (±25%)
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		delay = delay + jitter - (delay / 4)

		// Log retry attempt (would be picked up by logger if verbose mode enabled)
		// fmt.Printf("Retry %d/%d for %s after %v: %v\n", attempt, maxAttempts, operation, delay, err)
		time.Sleep(delay)
	}

	return lastErr // unreachable, but makes linter happy
}

// isRetryableError determines if an error should be retried.
// Transient network errors and temporary server issues are retryable.
// Validation errors and permanent failures are not.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Non-retryable patterns (fail fast)
	nonRetryable := []string{
		"400",
		"401",
		"403",
		"404",
		"duplicate",
		"invalid",
		"not found",
		"validation",
	}

	for _, pattern := range nonRetryable {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Retryable patterns (transient failures)
	retryable := []string{
		"eof",
		"connection reset",
		"closed network connection",
		"broken pipe",
		"500",
		"502",
		"503",
		"504",
		"timeout",
		"temporary failure",
	}

	for _, pattern := range retryable {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Default to non-retryable for unknown errors
	return false
}
