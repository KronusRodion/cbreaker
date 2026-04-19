package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"go.uber.org/zap"
)

// SlogObserver logs events using log/slog
type SlogObserver struct {
	logger *slog.Logger
}

func NewSlogObserver(logger *slog.Logger) *SlogObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlogObserver{logger: logger}
}

func (o *SlogObserver) OnStateChange(ctx context.Context, name string, fromState, toState domain.State) {
	o.logger.InfoContext(ctx, "circuit breaker state changed",
		slog.String("circuit", name),
		slog.String("from", fromState.String()),
		slog.String("to", toState.String()),
	)
}

func (o *SlogObserver) OnCall(ctx context.Context, name string, duration time.Duration, err error) {
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
	}

	o.logger.Log(ctx, level, "circuit breaker call completed",
		slog.String("circuit", name),
		slog.Duration("duration", duration),
		slog.Any("error", err),
	)
}

func (o *SlogObserver) OnSuccess(ctx context.Context, name string, duration time.Duration) {
	o.logger.DebugContext(ctx, "circuit breaker call succeeded",
		slog.String("circuit", name),
		slog.Duration("duration", duration),
	)
}

func (o *SlogObserver) OnFailure(ctx context.Context, name string, err error) {
	o.logger.WarnContext(ctx, "circuit breaker call failed",
		slog.String("circuit", name),
		slog.Any("error", err),
	)
}

func (o *SlogObserver) OnRejected(ctx context.Context, name string) {
	o.logger.WarnContext(ctx, "circuit breaker rejected call (circuit open)",
		slog.String("circuit", name),
	)
}

// ZapObserver for Uber's zap logger
type ZapObserver struct {
	logger *zap.Logger
}

func NewZapObserver(logger *zap.Logger) *ZapObserver {
	if logger == nil {
		logger = zap.L()
	}
	return &ZapObserver{logger: logger}
}

func (o *ZapObserver) OnStateChange(ctx context.Context, name string, fromState, toState domain.State) {
	o.logger.Info("circuit breaker state changed",
		zap.String("circuit", name),
		zap.String("from", fromState.String()),
		zap.String("to", toState.String()),
	)
}

func (o *ZapObserver) OnCall(ctx context.Context, name string, duration time.Duration, err error) {
	if err != nil {
		o.logger.Error("circuit breaker call failed",
			zap.String("circuit", name),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
	} else {
		o.logger.Debug("circuit breaker call succeeded",
			zap.String("circuit", name),
			zap.Duration("duration", duration),
		)
	}
}

func (o *ZapObserver) OnSuccess(ctx context.Context, name string, duration time.Duration) {
	// Already handled in OnCall, but can be used separately
}

func (o *ZapObserver) OnFailure(ctx context.Context, name string, err error) {
	// Already handled in OnCall
}

func (o *ZapObserver) OnRejected(ctx context.Context, name string) {
	o.logger.Warn("circuit breaker rejected call (circuit open)",
		zap.String("circuit", name),
	)
}
