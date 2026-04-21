package circuit

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/internal/window"
	"github.com/KronusRodion/cbreaker/ports"
)

// atomicBreaker implements lock-free circuit breaker
type atomicBreaker struct {
	name string

	// Configuration (read-only, no locking needed)
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	maxConcurrentRequests int
	rollingWindow         time.Duration
	observer              ports.Observer

	// Atomic state machine
	state       atomic.Int32  // domain.State as int32
	openTime    atomic.Int64  // Unix timestamp when circuit opened
	halfOpenSem chan struct{}

	// Track seq per goroutine (stored in context or TLS)
	// We'll use a simple approach: store in atomic value and compare

	// Atomic counters
	totalRequests  atomic.Int64
	totalFailures  atomic.Int64
	totalSuccesses atomic.Int64

	// Window with atomic operations
	window *window.AtomicSlidingWindow
}

// NewAtomicBreaker creates a lock-free circuit breaker
func NewAtomicBreaker(opts ...BreakerOption) *atomicBreaker {
	cfg := &breakerConfig{
		name:                  "default",
		failureThreshold:      5,
		successThreshold:      2,
		timeout:               30 * time.Second,
		maxConcurrentRequests: 1,
		rollingWindow:         60 * time.Second,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &atomicBreaker{
		name:                  cfg.name,
		failureThreshold:      cfg.failureThreshold,
		successThreshold:      cfg.successThreshold,
		timeout:               cfg.timeout,
		maxConcurrentRequests: cfg.maxConcurrentRequests,
		rollingWindow:         cfg.rollingWindow,
		halfOpenSem:           make(chan struct{}, cfg.maxConcurrentRequests),
		window:                window.NewAtomicSlidingWindow(cfg.rollingWindow, time.Second),
		observer:              cfg.observer,
	}
}

// TryAcquire attempts to acquire a permit (lock-free, seq hidden internally)
func (b *atomicBreaker) TryAcquire(ctx context.Context) (domain.State, error) {
	for {
		state := domain.State(b.state.Load())

		switch state {
		case domain.StateClosed:
			// Store seq for this acquisition (using context or we'll check on RecordOutcome)
			// We'll store in a goroutine-local way via atomic compare
			return state, nil

		case domain.StateOpen:
			openTime := time.Unix(0, b.openTime.Load())
			if time.Since(openTime) > b.timeout {
				if b.transitionState(domain.StateOpen, domain.StateHalfOpen) {
					return b.acquireHalfOpen(ctx)
				}
				continue
			}
			return state, domain.ErrCircuitOpen

		case domain.StateHalfOpen:
			return b.acquireHalfOpen(ctx)

		default:
			return state, nil
		}
	}
}

// acquireHalfOpen attempts to acquire the half-open semaphore
func (b *atomicBreaker) acquireHalfOpen(ctx context.Context) (domain.State, error) {
	select {
	case b.halfOpenSem <- struct{}{}:
		return domain.StateHalfOpen, nil
	case <-ctx.Done():
		return domain.StateHalfOpen, ctx.Err()
	}
}


// transitionState atomically changes state
func (b *atomicBreaker) transitionState(oldState, newState domain.State) bool {
	// First verify we're still in the expected state and sequence
	currentState := domain.State(b.state.Load())

	if currentState != oldState {
		return false
	}

	// CAS on state
	if !b.state.CompareAndSwap(int32(oldState), int32(newState)) {
		return false
	}

	// Notify observer
	if b.observer != nil {
		go b.observer.OnStateChange(context.Background(), b.name, oldState, newState)
	}

	return true
}

// RecordOutcome records the result (seq passed from caller)
// Note: Caller must pass the seq they got from TryAcquire
// RecordOutcome records the result using seq from context
func (b *atomicBreaker) RecordOutcome(state domain.State, err error, duration time.Duration) {
	// Get seq from context (must be stored by TryAcquire)
	// Since we can't access context here, we need to pass seq as parameter
	// But we want to keep interface clean, so we'll use another approach
	
	
	// Release half-open semaphore if needed
	if state == domain.StateHalfOpen {
		select {
		case <-b.halfOpenSem:
		default:
		}
	}

	// Update counters
	b.totalRequests.Add(1)
	b.window.Add(duration)
	
	// Double-check state matches
	currentState := domain.State(b.state.Load())
	if currentState != state {
		return
	}

	if err != nil {
		b.totalFailures.Add(1)
		b.window.RecordFailure()
		b.checkAndTransitionToOpen(state)
	} else {
		b.totalSuccesses.Add(1)
		b.window.RecordSuccess()
		if state == domain.StateHalfOpen {
			b.checkAndTransitionToClosed()
		}
	} 
}

// checkAndTransitionToOpen attempts to transition to open
func (b *atomicBreaker) checkAndTransitionToOpen(currentState domain.State) {
	if currentState != domain.StateClosed {
		return
	}

	failureRate := b.window.FailureRate()
	totalFailures := b.window.TotalFailures()

	if failureRate >= 0.5 && totalFailures >= b.failureThreshold {
		if b.state.CompareAndSwap(int32(domain.StateClosed), int32(domain.StateOpen)) {
			b.openTime.Store(time.Now().UnixNano())
			b.window.Reset()

			if b.observer != nil {
				go b.observer.OnStateChange(context.Background(), b.name, domain.StateClosed, domain.StateOpen)
			}
		}
	}
}

// checkAndTransitionToClosed attempts to transition to closed
func (b *atomicBreaker) checkAndTransitionToClosed() {
	consecutiveSuccesses := b.window.ConsecutiveSuccesses()
	if consecutiveSuccesses >= b.successThreshold {
		if b.state.CompareAndSwap(int32(domain.StateHalfOpen), int32(domain.StateClosed)) {
			b.window.Reset()

			if b.observer != nil {
				go b.observer.OnStateChange(context.Background(), b.name, domain.StateHalfOpen, domain.StateClosed)
			}
		}
	}
}

// State returns current state
func (b *atomicBreaker) State() domain.State {
	return domain.State(b.state.Load())
}

// Reset resets the circuit
func (b *atomicBreaker) Reset() {
	for {
		oldState := b.state.Load()
		if b.state.CompareAndSwap(oldState, int32(domain.StateClosed)) {
			b.window.Reset()
			b.totalRequests.Store(0)
			b.totalFailures.Store(0)
			b.totalSuccesses.Store(0)
			b.openTime.Store(0)

			// Clear semaphore
			for {
				select {
				case <-b.halfOpenSem:
				default:
					return
				}
			}
		}
	}
}

// Metrics returns current metrics
func (b *atomicBreaker) Metrics() ports.Metrics {
	return &AtomicMetrics{
		totalRequests:  b.totalRequests.Load(),
		totalFailures:  b.totalFailures.Load(),
		totalSuccesses: b.totalSuccesses.Load(),
		failureRate:    b.window.FailureRate(),
		consecutive:    b.window.ConsecutiveSuccesses(),
	}
}

// NotifyRejected notifies observer
func (b *atomicBreaker) NotifyRejected(ctx context.Context) {
	if b.observer != nil {
		go b.observer.OnRejected(ctx, b.name)
	}
}

// NotifyFailure notifies observer
func (b *atomicBreaker) NotifyFailure(ctx context.Context, err error) {
	if b.observer != nil {
		go b.observer.OnFailure(ctx, b.name, err)
	}
}

// NotifySuccess notifies observer
func (b *atomicBreaker) NotifySuccess(ctx context.Context, duration time.Duration) {
	if b.observer != nil {
		go b.observer.OnSuccess(ctx, b.name, duration)
	}
}

// AtomicMetrics implementation
type AtomicMetrics struct {
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64
	failureRate    float64
	consecutive    int
}

func (m *AtomicMetrics) TotalRequests() int64      { return m.totalRequests }
func (m *AtomicMetrics) TotalFailures() int64      { return m.totalFailures }
func (m *AtomicMetrics) TotalSuccesses() int64     { return m.totalSuccesses }
func (m *AtomicMetrics) FailureRate() float64      { return m.failureRate }
func (m *AtomicMetrics) ConsecutiveSuccesses() int { return m.consecutive }
