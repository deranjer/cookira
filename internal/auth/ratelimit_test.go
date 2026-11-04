package auth

import (
	"testing"
	"time"
)

func TestWindowLimiter(t *testing.T) {
	l := NewWindowLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("request %d denied within limit", i+1)
		}
	}
	if l.Allow("a") {
		t.Error("4th request allowed")
	}
	if !l.Allow("b") {
		t.Error("other key throttled")
	}
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("a") {
		t.Error("window did not reset")
	}
}
