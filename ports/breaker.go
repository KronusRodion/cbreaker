package ports

import (
	"context"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
)

// Breaker interface
type Breaker interface {
	TryAcquire(ctx context.Context) (domain.State, error)

	NotifyRejected(ctx context.Context)
	NotifyFailure(ctx context.Context, err error)
	RecordOutcome( state domain.State, err error, duration time.Duration)
	NotifySuccess(ctx context.Context, duration time.Duration)

	State() domain.State
	Metrics() Metrics
	Reset()
}
