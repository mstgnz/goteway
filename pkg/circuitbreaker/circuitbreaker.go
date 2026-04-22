package circuitbreaker

import (
	"sync"
	"time"

	"github.com/mstgnz/goteway/pkg/logger"
)

// State represents the circuit breaker state.
type State int

const (
	// StateClosed is normal operation — requests flow through.
	StateClosed State = iota
	// StateOpen means the backend is considered down — requests are rejected immediately.
	StateOpen
	// StateHalfOpen allows one probe request to test whether the backend recovered.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker is a thread-safe implementation of the circuit breaker pattern.
// It transitions Closed → Open after [Threshold] consecutive failures, then
// moves to HalfOpen after [OpenTimeout] and back to Closed on the first success.
type CircuitBreaker struct {
	name        string
	threshold   int
	openTimeout time.Duration

	mu              sync.Mutex
	state           State
	failures        int
	lastStateChange time.Time

	log *logger.Logger
}

// New creates a circuit breaker.
//   - name: used in log messages
//   - threshold: consecutive failures before opening
//   - openTimeout: how long to stay open before probing (half-open)
func New(name string, threshold int, openTimeout time.Duration, log *logger.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		threshold:       threshold,
		openTimeout:     openTimeout,
		state:           StateClosed,
		lastStateChange: time.Now(),
		log:             log,
	}
}

// Allow returns true if the request should be forwarded to the backend.
// It transitions StateOpen → StateHalfOpen when the open timeout elapses.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastStateChange) >= cb.openTimeout {
			cb.state = StateHalfOpen
			cb.lastStateChange = time.Now()
			cb.log.Info("Circuit breaker %q: half-open, probing backend", cb.name)
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess notifies the circuit breaker that the last request succeeded.
// While HalfOpen, one success closes the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.state = StateClosed
		cb.failures = 0
		cb.lastStateChange = time.Now()
		cb.log.Info("Circuit breaker %q: closed — backend recovered", cb.name)
	case StateClosed:
		cb.failures = 0
	}
}

// RecordFailure notifies the circuit breaker that the last request failed.
// After [threshold] failures the circuit opens; a failure in HalfOpen reopens it.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.state == StateHalfOpen || cb.failures >= cb.threshold {
		cb.state = StateOpen
		cb.lastStateChange = time.Now()
		cb.log.Warn("Circuit breaker %q: open after %d failure(s)", cb.name, cb.failures)
	}
}

// State returns the current state (safe for concurrent use).
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
