package metrics

import (
	"context"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusObserver struct {

	// Gauges
	stateGauge *prometheus.GaugeVec

	// Counters
	callsTotal     *prometheus.CounterVec
	successesTotal *prometheus.CounterVec
	failuresTotal  *prometheus.CounterVec
	rejectedTotal  *prometheus.CounterVec

	// Histograms
	callDuration *prometheus.HistogramVec
}

// PrometheusConfig allows customization
type PrometheusConfig struct {
	Namespace       string
	Subsystem       string
	DurationBuckets []float64
}

func NewPrometheusObserver(cfg *PrometheusConfig) *PrometheusObserver {
	if cfg == nil {
		cfg = &PrometheusConfig{
			Namespace: "circlek",
			Subsystem: "circuit_breaker",
			DurationBuckets: []float64{
				0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
			},
		}
	}

	return &PrometheusObserver{
		stateGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "state",
				Help:      "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
			},
			[]string{"circuit"},
		),

		callsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "calls_total",
				Help:      "Total number of calls",
			},
			[]string{"circuit", "result"}, // result: success, failure, rejected
		),

		successesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "successes_total",
				Help:      "Total number of successful calls",
			},
			[]string{"circuit"},
		),

		failuresTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "failures_total",
				Help:      "Total number of failed calls",
			},
			[]string{"circuit"},
		),

		rejectedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "rejected_total",
				Help:      "Total number of rejected calls (circuit open)",
			},
			[]string{"circuit"},
		),

		callDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: cfg.Namespace,
				Subsystem: cfg.Subsystem,
				Name:      "call_duration_seconds",
				Help:      "Duration of calls",
				Buckets:   cfg.DurationBuckets,
			},
			[]string{"circuit", "result"},
		),
	}
}

// Register must be called to register metrics with Prometheus
func (o *PrometheusObserver) Register(registerer prometheus.Registerer) error {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	collectors := []prometheus.Collector{
		o.stateGauge,
		o.callsTotal,
		o.successesTotal,
		o.failuresTotal,
		o.rejectedTotal,
		o.callDuration,
	}

	for _, coll := range collectors {
		if err := registerer.Register(coll); err != nil {
			// Allow already registered errors
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				return err
			}
		}
	}

	return nil
}

func (o *PrometheusObserver) stateToGauge(state domain.State) float64 {
	switch state {
	case domain.StateClosed:
		return 0
	case domain.StateOpen:
		return 1
	case domain.StateHalfOpen:
		return 2
	default:
		return -1
	}
}

func (o *PrometheusObserver) OnStateChange(_ context.Context, name string, _, toState domain.State) {
	o.stateGauge.WithLabelValues(name).Set(o.stateToGauge(toState))
}

func (o *PrometheusObserver) OnCall(_ context.Context, name string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "failure"
	}

	o.callsTotal.WithLabelValues(name, result).Inc()
	o.callDuration.WithLabelValues(name, result).Observe(duration.Seconds())
}

func (o *PrometheusObserver) OnSuccess(_ context.Context, name string, _ time.Duration) {
	o.successesTotal.WithLabelValues(name).Inc()
}

func (o *PrometheusObserver) OnFailure(_ context.Context, name string, _ error) {
	o.failuresTotal.WithLabelValues(name).Inc()
}

func (o *PrometheusObserver) OnRejected(_ context.Context, name string) {
	o.rejectedTotal.WithLabelValues(name).Inc()
	o.callsTotal.WithLabelValues(name, "rejected").Inc()
}
