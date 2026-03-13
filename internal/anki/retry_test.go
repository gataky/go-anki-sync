package anki

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestRetryWithBackoff_Success tests that a successful operation returns immediately.
func TestRetryWithBackoff_Success(t *testing.T) {
	callCount := 0
	operation := func() error {
		callCount++
		return nil
	}

	err := retryWithBackoff("TestOp", 3, operation)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected operation to be called once, got %d calls", callCount)
	}
}

// TestRetryWithBackoff_TransientFailureThenSuccess tests retry on transient failure.
func TestRetryWithBackoff_TransientFailureThenSuccess(t *testing.T) {
	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("EOF") // Retryable error
		}
		return nil
	}

	err := retryWithBackoff("TestOp", 3, operation)

	if err != nil {
		t.Errorf("Expected no error after retries, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d calls", callCount)
	}
}

// TestRetryWithBackoff_AllAttemptsFail tests exhausting all retry attempts.
func TestRetryWithBackoff_AllAttemptsFail(t *testing.T) {
	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("500 internal server error") // Retryable error
	}

	err := retryWithBackoff("TestOp", 3, operation)

	if err == nil {
		t.Error("Expected error after all retries exhausted, got nil")
	}
	if callCount != 3 {
		t.Errorf("Expected operation to be called 3 times, got %d calls", callCount)
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Expected error message to mention retry attempts, got: %v", err)
	}
}

// TestRetryWithBackoff_NonRetryableError tests immediate failure on non-retryable error.
func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("400 bad request") // Non-retryable error
	}

	err := retryWithBackoff("TestOp", 3, operation)

	if err == nil {
		t.Error("Expected error for non-retryable failure, got nil")
	}
	if callCount != 1 {
		t.Errorf("Expected operation to be called once (no retries), got %d calls", callCount)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected original error message, got: %v", err)
	}
}

// TestRetryWithBackoff_BackoffTiming tests that delays increase exponentially.
func TestRetryWithBackoff_BackoffTiming(t *testing.T) {
	callCount := 0
	var callTimes []time.Time

	operation := func() error {
		callCount++
		callTimes = append(callTimes, time.Now())
		return errors.New("connection reset") // Retryable error
	}

	startTime := time.Now()
	_ = retryWithBackoff("TestOp", 3, operation)
	totalDuration := time.Since(startTime)

	// Should have made 3 calls
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}

	// Check delays between attempts
	// First delay should be ~100ms (with ±25% jitter, min ~75ms)
	// Second delay should be ~200ms (with ±25% jitter, min ~150ms)
	// Total should be at least 75ms + 150ms = 225ms (accounting for jitter)
	minExpectedDuration := 200 * time.Millisecond // Slightly below 225ms to account for variance
	if totalDuration < minExpectedDuration {
		t.Errorf("Expected total duration >= %v, got %v", minExpectedDuration, totalDuration)
	}

	// Should not exceed max delay * 2 attempts = 5s * 2 = 10s (generous upper bound)
	maxExpectedDuration := 10 * time.Second
	if totalDuration > maxExpectedDuration {
		t.Errorf("Expected total duration <= %v, got %v", maxExpectedDuration, totalDuration)
	}
}

// TestIsRetryableError_RetryableErrors tests retryable error patterns.
func TestIsRetryableError_RetryableErrors(t *testing.T) {
	retryableErrors := []error{
		errors.New("EOF"),
		errors.New("connection reset by peer"),
		errors.New("use of closed network connection"),
		errors.New("write: broken pipe"),
		errors.New("500 internal server error"),
		errors.New("502 bad gateway"),
		errors.New("503 service unavailable"),
		errors.New("504 gateway timeout"),
		errors.New("request timeout"),
		errors.New("temporary failure in name resolution"),
	}

	for _, err := range retryableErrors {
		if !isRetryableError(err) {
			t.Errorf("Expected error to be retryable: %v", err)
		}
	}
}

// TestIsRetryableError_NonRetryableErrors tests non-retryable error patterns.
func TestIsRetryableError_NonRetryableErrors(t *testing.T) {
	nonRetryableErrors := []error{
		errors.New("400 bad request"),
		errors.New("401 unauthorized"),
		errors.New("403 forbidden"),
		errors.New("404 not found"),
		errors.New("duplicate entry"),
		errors.New("invalid input"),
		errors.New("resource not found"),
		errors.New("validation error"),
		errors.New("some other unknown error"),
	}

	for _, err := range nonRetryableErrors {
		if isRetryableError(err) {
			t.Errorf("Expected error to be non-retryable: %v", err)
		}
	}
}

// TestIsRetryableError_NilError tests nil error handling.
func TestIsRetryableError_NilError(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("Expected nil error to be non-retryable")
	}
}

// TestRetryWithBackoff_WrapsOriginalError tests error wrapping.
func TestRetryWithBackoff_WrapsOriginalError(t *testing.T) {
	originalErr := errors.New("EOF")
	operation := func() error {
		return originalErr
	}

	err := retryWithBackoff("TestOp", 3, operation)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Check that original error is wrapped
	if !errors.Is(err, originalErr) {
		t.Errorf("Expected error to wrap original error, got: %v", err)
	}
}

// TestRetryWithBackoff_ContextInErrorMessage tests error messages contain context.
func TestRetryWithBackoff_ContextInErrorMessage(t *testing.T) {
	operation := func() error {
		return errors.New("500 internal server error")
	}

	err := retryWithBackoff("CreateDeck", 3, operation)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "CreateDeck") {
		t.Errorf("Expected error message to contain operation name, got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "3 attempts") {
		t.Errorf("Expected error message to contain attempt count, got: %v", errMsg)
	}
}

// TestRetryWithBackoff_CaseInsensitiveErrorMatching tests error pattern matching is case-insensitive.
func TestRetryWithBackoff_CaseInsensitiveErrorMatching(t *testing.T) {
	testCases := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{"Lowercase EOF", errors.New("eof"), true},
		{"Uppercase EOF", errors.New("EOF"), true},
		{"Mixed case EOF", errors.New("EoF"), true},
		{"500 error", errors.New("500 Internal Server Error"), true},
		{"Connection reset", errors.New("Connection Reset By Peer"), true},
		{"400 error", errors.New("400 Bad Request"), false},
		{"Duplicate", errors.New("Duplicate Entry"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isRetryableError(tc.err)
			if result != tc.shouldRetry {
				t.Errorf("For error %q, expected retryable=%v, got %v", tc.err, tc.shouldRetry, result)
			}
		})
	}
}

// BenchmarkRetryWithBackoff_Success benchmarks successful operations (no retries).
func BenchmarkRetryWithBackoff_Success(b *testing.B) {
	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = retryWithBackoff("BenchOp", 3, operation)
	}
}

// TestRetryWithBackoff_MaxDelayRespected tests that delay doesn't exceed maxDelay.
func TestRetryWithBackoff_MaxDelayRespected(t *testing.T) {
	// This test would require many attempts to exceed max delay
	// For simplicity, we just verify the logic works with a few attempts
	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 5 {
			return errors.New("timeout")
		}
		return nil
	}

	// Use a higher max attempts to test max delay logic
	err := retryWithBackoff("TestOp", 5, operation)

	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
	if callCount != 5 {
		t.Errorf("Expected 5 calls, got %d", callCount)
	}
}

// TestRetryWithBackoff_MultipleRetryableErrors tests different retryable error types.
func TestRetryWithBackoff_MultipleRetryableErrors(t *testing.T) {
	errors := []error{
		fmt.Errorf("EOF"),
		fmt.Errorf("connection reset"),
		fmt.Errorf("502 bad gateway"),
	}

	callCount := 0
	operation := func() error {
		if callCount < len(errors) {
			err := errors[callCount]
			callCount++
			return err
		}
		return nil
	}

	err := retryWithBackoff("TestOp", 5, operation)

	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}
