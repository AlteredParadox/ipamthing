package auth

import (
	"testing"
	"time"
)

const testIP = "10.0.0.1"

func TestLoginLimiterAllowsUnderThreshold(t *testing.T) {
	l := NewLoginLimiter(3, time.Minute, time.Minute)
	for i := 0; i < 3; i++ {
		ok, _ := l.Allow("192.168.1.1")
		if !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		l.Fail("192.168.1.1")
	}
}

func TestLoginLimiterLocksOut(t *testing.T) {
	l := NewLoginLimiter(3, time.Minute, time.Minute)
	for i := 0; i < 3; i++ {
		l.Fail(testIP)
	}
	ok, retryAfter := l.Allow(testIP)
	if ok {
		t.Fatal("should be locked out after 3 failures")
	}
	if retryAfter <= 0 {
		t.Error("retryAfter should be positive")
	}
}

func TestLoginLimiterSuccessResets(t *testing.T) {
	l := NewLoginLimiter(3, time.Minute, time.Minute)
	l.Fail(testIP)
	l.Fail(testIP)
	l.Success(testIP)

	ok, _ := l.Allow(testIP)
	if !ok {
		t.Fatal("should be allowed after Success resets the counter")
	}
}

func TestLoginLimiterIsolatesKeys(t *testing.T) {
	l := NewLoginLimiter(2, time.Minute, time.Minute)
	l.Fail(testIP)
	l.Fail(testIP)

	ok, _ := l.Allow("10.0.0.2")
	if !ok {
		t.Fatal("different key should not be locked out")
	}
}

func TestLoginLimiterWindowExpiry(t *testing.T) {
	l := NewLoginLimiter(3, 1*time.Millisecond, time.Minute)
	l.Fail(testIP)
	l.Fail(testIP)
	time.Sleep(5 * time.Millisecond)

	ok, _ := l.Allow(testIP)
	if !ok {
		t.Fatal("should be allowed after window expires")
	}
}
