package ports

import (
	"context"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
)

type Observer interface {
	// OnStateChange is called when circuit breaker state changes
	OnStateChange(ctx context.Context, name string, fromState, toState domain.State)

	// OnCall is called when a call completes (success or failure)
	OnCall(ctx context.Context, name string, duration time.Duration, err error)

	// OnSuccess is called when a call succeeds
	OnSuccess(ctx context.Context, name string, duration time.Duration)

	// OnFailure is called when a call fails
	OnFailure(ctx context.Context, name string, err error)

	// OnRejected is called when a call is rejected because circuit is open
	OnRejected(ctx context.Context, name string)
}
