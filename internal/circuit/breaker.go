package circuit

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/internal/window"
	"github.com/KronusRodion/cbreaker/ports"
)

// internal implementation
type BreakerImpl struct {
	name string
	mu   sync.RWMutex

	// Configuration
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	maxConcurrentRequests int
	rollingWindow         time.Duration

	// State
	state       domain.State
	openTime    time.Time
	halfOpenSem chan struct{}

	// Metrics
	totalRequests  atomic.Int64
	totalFailures  atomic.Int64
	totalSuccesses atomic.Int64

	// Sliding window
	window *window.SlidingWindow

	// Observer
	observer ports.Observer
}

type breakerConfig struct {
	name                  string
	failureThreshold      int
	successThreshold      int
	timeout               time.Duration
	maxConcurrentRequests int
	rollingWindow         time.Duration
	observer              ports.Observer
}

type BreakerOption func(*breakerConfig)

func WithName(name string) BreakerOption {
	return func(c *breakerConfig) { c.name = name }
}

func WithFailureThreshold(n int) BreakerOption {
	return func(c *breakerConfig) { c.failureThreshold = n }
}

func WithSuccessThreshold(n int) BreakerOption {
	return func(c *breakerConfig) { c.successThreshold = n }
}

func WithTimeout(d time.Duration) BreakerOption {
	return func(c *breakerConfig) { c.timeout = d }
}

func WithMaxConcurrentRequests(n int) BreakerOption {
	return func(c *breakerConfig) { c.maxConcurrentRequests = n }
}

func WithRollingWindow(d time.Duration) BreakerOption {
	return func(c *breakerConfig) { c.rollingWindow = d }
}

func WithObserver(obs ports.Observer) BreakerOption {
	return func(c *breakerConfig) { c.observer = obs }
}

func NewBreaker(opts ...BreakerOption) *BreakerImpl {
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

	return &BreakerImpl{
		name:                  cfg.name,
		failureThreshold:      cfg.failureThreshold,
		successThreshold:      cfg.successThreshold,
		timeout:               cfg.timeout,
		maxConcurrentRequests: cfg.maxConcurrentRequests,
		rollingWindow:         cfg.rollingWindow,
		halfOpenSem:           make(chan struct{}, cfg.maxConcurrentRequests),
		window:                window.NewSlidingWindow(cfg.rollingWindow, time.Second),
		observer:              cfg.observer,
		state:                 domain.StateClosed,
	}
}

func (b *BreakerImpl) TryAcquire(ctx context.Context) (domain.State, error) {
	b.mu.RLock()
	currentState := b.state
	b.mu.RUnlock()

	switch currentState {
	case domain.StateClosed:
		return currentState, nil

	case domain.StateOpen:
		b.mu.Lock()
		if time.Since(b.openTime) > b.timeout {
			b.setState(domain.StateHalfOpen)
			currentState = domain.StateHalfOpen
		} else {
			b.mu.Unlock()
			return currentState, domain.ErrCircuitOpen
		}
		b.mu.Unlock()
		fallthrough

	case domain.StateHalfOpen:
		select {
		case b.halfOpenSem <- struct{}{}:
			return currentState, nil
		case <-ctx.Done():
			return currentState, ctx.Err()
		}
	}

	return currentState, nil
}

func (b *BreakerImpl) RecordOutcome(state domain.State, err error, duration time.Duration) {
	// Освобождаем семафор если нужно
	if state == domain.StateHalfOpen {
		<-b.halfOpenSem
	}
	// Обновляем счетчики
	b.totalRequests.Add(1)
	b.window.Add(duration)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != state {
		// Состояние изменилось, игнорируем результат
		return
	}

	if err != nil {
		b.totalFailures.Add(1)
		b.window.RecordFailure()
		if b.state == domain.StateClosed && b.shouldTrip() {
			b.setState(domain.StateOpen)
			b.openTime = time.Now()
		}
	} else {
		b.totalSuccesses.Add(1)
		b.window.RecordSuccess()
		if b.state == domain.StateHalfOpen {
			if b.window.ConsecutiveSuccesses() >= b.successThreshold {
				b.setState(domain.StateClosed)
				b.window.Reset()
			}
		}
	}
}

func (b *BreakerImpl) shouldTrip() bool {
	failureRate := b.window.FailureRate()
	return failureRate >= 0.5 && b.window.TotalFailures() >= b.failureThreshold
}

func (b *BreakerImpl) setState(newState domain.State) {

	oldState := b.state
	b.state = newState

	// Notify observer
	if b.observer != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("observer panicked: %v", r)
				}
			}()
			b.observer.OnStateChange(context.Background(), b.name, oldState, newState)
		}()
	}
}

func (b *BreakerImpl) State() domain.State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *BreakerImpl) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.setState(domain.StateClosed)
	b.window.Reset()
	b.totalRequests.Store(0)
	b.totalFailures.Store(0)
	b.totalSuccesses.Store(0)
}

func (b *BreakerImpl) Metrics() ports.Metrics {
	return &MetricsImpl{
		totalRequests:  b.totalRequests.Load(),
		totalFailures:  b.totalFailures.Load(),
		totalSuccesses: b.totalSuccesses.Load(),
		failureRate:    b.window.FailureRate(),
		consecutive:    b.window.ConsecutiveSuccesses(),
	}
}

// Helper methods for observer notifications
func (b *BreakerImpl) NotifyRejected(ctx context.Context) {
	if b.observer != nil {
		b.observer.OnRejected(ctx, b.name)
	}
}

func (b *BreakerImpl) NotifyFailure(ctx context.Context, err error) {
	if b.observer != nil {
		b.observer.OnFailure(ctx, b.name, err)
	}
}

func (b *BreakerImpl) NotifySuccess(ctx context.Context, duration time.Duration) {
	if b.observer != nil {
		b.observer.OnSuccess(ctx, b.name, duration)
	}
}

type MetricsImpl struct {
	totalRequests  int64
	totalFailures  int64
	totalSuccesses int64
	failureRate    float64
	consecutive    int
}

func (m *MetricsImpl) TotalRequests() int64      { return m.totalRequests }
func (m *MetricsImpl) TotalFailures() int64      { return m.totalFailures }
func (m *MetricsImpl) TotalSuccesses() int64     { return m.totalSuccesses }
func (m *MetricsImpl) FailureRate() float64      { return m.failureRate }
func (m *MetricsImpl) ConsecutiveSuccesses() int { return m.consecutive }
