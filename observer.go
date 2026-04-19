package cbreaker

import (
	"context"
	"log/slog"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/metrics"
	"github.com/KronusRodion/cbreaker/ports"
)

// DefaultObserver - возвращает наблюдателя с slog
func DefaultObserver() ports.Observer {
	logObserver := metrics.NewSlogObserver(slog.Default())
	return NewMultiObserver(logObserver)
}

// MultiObserver combines multiple observers
type MultiObserver struct {
	observers []ports.Observer
}

func NewMultiObserver(observers ...ports.Observer) *MultiObserver {
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
