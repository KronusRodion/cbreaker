package metrics

import (
	"context"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type OtelObserver struct {
	meter metric.Meter

	// Instruments
	stateGauge   metric.Int64Gauge
	callsCounter metric.Int64Counter
	durationHist metric.Float64Histogram
}

func NewOtelObserver(meter metric.Meter) (*OtelObserver, error) {
	if meter == nil {
		meter = otel.Meter("circlek")
	}

	stateGauge, err := meter.Int64Gauge("circuit.state")
	if err != nil {
		return nil, err
	}

	callsCounter, err := meter.Int64Counter("circuit.calls.total")
	if err != nil {
		return nil, err
	}

	durationHist, err := meter.Float64Histogram("circuit.call.duration")
	if err != nil {
		return nil, err
	}

	return &OtelObserver{
		meter:        meter,
		stateGauge:   stateGauge,
		callsCounter: callsCounter,
		durationHist: durationHist,
	}, nil
}

func (o *OtelObserver) OnStateChange(ctx context.Context, name string, fromState, toState domain.State) {
	o.stateGauge.Record(ctx, int64(toState),
		metric.WithAttributes(
			attribute.String("circuit", name),
			attribute.String("from", fromState.String()),
		),
	)
}

func (o *OtelObserver) OnCall(ctx context.Context, name string, duration time.Duration, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("circuit", name),
	}

	if err != nil {
		attrs = append(attrs, attribute.Bool("error", true))
		attrs = append(attrs, attribute.String("error_type", err.Error()))
	}

	o.callsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	o.durationHist.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func (o *OtelObserver) OnSuccess(_ context.Context, _ string, _ time.Duration) {
	// Already handled in OnCall
}

func (o *OtelObserver) OnFailure(_ context.Context, _ string, _ error) {
	// Already handled in OnCall
}

func (o *OtelObserver) OnRejected(ctx context.Context, name string) {
	o.callsCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("circuit", name),
			attribute.String("reason", "circuit_open"),
		),
	)
}
