package auth

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, 100*time.Millisecond)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !rl.Allow("test-key") {
			t.Errorf("Allow() returned false for request %d, want true", i+1)
		}
	}

	// 4th request should be denied
	if rl.Allow("test-key") {
		t.Error("Allow() returned true for 4th request, want false")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	// Use up the limit
	rl.Allow("test-key")
	rl.Allow("test-key")

	// Should be denied
	if rl.Allow("test-key") {
		t.Error("Allow() returned true after limit, want false")
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow("test-key") {
		t.Error("Allow() returned false after window reset, want true")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(1, 100*time.Millisecond)

	// First key should be allowed
	if !rl.Allow("key1") {
		t.Error("Allow() returned false for key1, want true")
	}

	// Second request for first key should be denied
	if rl.Allow("key1") {
		t.Error("Allow() returned true for key1 second request, want false")
	}

	// Different key should be allowed
	if !rl.Allow("key2") {
		t.Error("Allow() returned false for key2, want true")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, 100*time.Millisecond)

	// Use up the limit
	rl.Allow("test-key")

	// Should be denied
	if rl.Allow("test-key") {
		t.Error("Allow() returned true after limit, want false")
	}

	// Reset the key
	rl.Reset("test-key")

	// Should be allowed again
	if !rl.Allow("test-key") {
		t.Error("Allow() returned false after Reset(), want true")
	}
}
