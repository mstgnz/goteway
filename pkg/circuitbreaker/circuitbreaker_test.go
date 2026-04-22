package circuitbreaker

import (
	"testing"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

func newCB(threshold int, openTimeout time.Duration) *CircuitBreaker {
	return New("test", threshold, openTimeout, logger.New(logger.INFO))
}

func TestInitialStateClosed(t *testing.T) {
	cb := newCB(3, time.Second)
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want closed", cb.State())
	}
	if !cb.Allow() {
		t.Error("Allow() should return true when closed")
	}
}

func TestOpensAfterThreshold(t *testing.T) {
	cb := newCB(3, time.Second)
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}
	if cb.State() != StateOpen {
		t.Errorf("state after threshold failures = %v, want open", cb.State())
	}
	if cb.Allow() {
		t.Error("Allow() should return false when open")
	}
}

func TestTransitionsHalfOpenAfterTimeout(t *testing.T) {
	cb := newCB(1, 50*time.Millisecond)
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected open, got %v", cb.State())
	}

	time.Sleep(60 * time.Millisecond)

	if !cb.Allow() {
		t.Error("Allow() should return true after open timeout (half-open)")
	}
	if cb.State() != StateHalfOpen {
		t.Errorf("state after timeout = %v, want half-open", cb.State())
	}
}

func TestClosesOnSuccessInHalfOpen(t *testing.T) {
	cb := newCB(1, 50*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Errorf("state after success in half-open = %v, want closed", cb.State())
	}
}

func TestReopensOnFailureInHalfOpen(t *testing.T) {
	cb := newCB(1, 50*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("state after failure in half-open = %v, want open", cb.State())
	}
}

func TestSuccessResetFailuresWhenClosed(t *testing.T) {
	cb := newCB(3, time.Second)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets counter
	cb.RecordFailure()
	cb.RecordFailure()
	// only 2 failures since last reset — circuit should still be closed
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed after success reset", cb.State())
	}
}

func TestStateString(t *testing.T) {
	cases := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.state.String(); got != c.want {
			t.Errorf("State(%d).String() = %q, want %q", c.state, got, c.want)
		}
	}
}
