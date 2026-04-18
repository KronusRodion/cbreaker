package observer

import (
	"context"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
)

// Observer defines the interface for circuit breaker observability

// NoopObserver does nothing (default)
type NoopObserver struct{}

func (n *NoopObserver) OnStateChange(ctx context.Context, name string, fromState, toState domain.State) {
}
func (n *NoopObserver) OnCall(ctx context.Context, name string, duration time.Duration, err error) {}
func (n *NoopObserver) OnSuccess(ctx context.Context, name string, duration time.Duration)         {}
func (n *NoopObserver) OnFailure(ctx context.Context, name string, err error)                      {}
func (n *NoopObserver) OnRejected(ctx context.Context, name string)                                {}

// MultiObserver combines multiple observers
type MultiObserver struct {
	observers []NoopObserver
}

func NewMultiObserver(observers ...NoopObserver) *MultiObserver {
	return &MultiObserver{observers: observers}
}

func (m *MultiObserver) OnStateChange(ctx context.Context, name string, fromState, toState domain.State) {
	for _, obs := range m.observers {
		obs.OnStateChange(ctx, name, fromState, toState)
	}
}

func (m *MultiObserver) OnCall(ctx context.Context, name string, duration time.Duration, err error) {
	for _, obs := range m.observers {
		obs.OnCall(ctx, name, duration, err)
	}
}

func (m *MultiObserver) OnSuccess(ctx context.Context, name string, duration time.Duration) {
	for _, obs := range m.observers {
		obs.OnSuccess(ctx, name, duration)
	}
}

func (m *MultiObserver) OnFailure(ctx context.Context, name string, err error) {
	for _, obs := range m.observers {
		obs.OnFailure(ctx, name, err)
	}
}

func (m *MultiObserver) OnRejected(ctx context.Context, name string) {
	for _, obs := range m.observers {
		obs.OnRejected(ctx, name)
	}
}
