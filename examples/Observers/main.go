package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KronusRodion/cbreaker"
	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/metrics"
)

type user struct {
	name string
	age  int
}

func main() {

	// Create Prometheus observer
	promObserver := metrics.NewPrometheusObserver(&metrics.PrometheusConfig{
		Namespace: "myapp",
		Subsystem: "circuit_breaker",
	})
	// use default registry
	if err := promObserver.Register(nil); err != nil {
		log.Printf("Warning: Prometheus registration failed: %v", err)
	}

	// Create OpenTelemetry observer (use default meter)
	otelObserver, err := metrics.NewOtelObserver(nil)
	if err != nil {
		log.Printf("Warning: OpenTelemetry observer creation failed: %v", err)
	}

	// Create Slog observer
	slogObserver := metrics.NewSlogObserver(nil)

	// Combine all observers
	multiObserver := cbreaker.NewMultiObserver(
		promObserver,
		otelObserver,
		slogObserver,
	)

	// Configure circuit breaker with created observer
	cbreaker.Configure(&cbreaker.Config{
		Name:                  "my-service",
		FailureThreshold:      3,
		SuccessThreshold:      2,
		Timeout:               5 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         30 * time.Second,
		Observer:              multiObserver,
	})

	cbreaker.AddResources("postgres")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	getUser := func(ctx context.Context) (user, error) {
		v := rand.Intn(100)
		if v < 80 {
			return user{}, errors.New("connection close")
		}
		return user{name: "Rodni", age: 11}, nil
	}

	var wg sync.WaitGroup

	errStat := atomic.Int32{}
	rejectStat := atomic.Int32{}
	successStat := atomic.Int32{}

	// use func Call from packet github.com/KronusRodion/cbreaker
	for range 100 {
		wg.Go(func() {
			_, err := cbreaker.Call(ctx, "postgres", getUser)
			if err != nil {
				switch {
				case errors.Is(err, domain.ErrCircuitOpen):
					rejectStat.Add(1)
				default:
					errStat.Add(1)
				}
			} else {
				successStat.Add(1)
			}
		})
	}

	wg.Wait()

	log.Println("Errors: ", errStat.Load())
	log.Println("Rejected: ", rejectStat.Load())
	log.Println("Success: ", successStat.Load())

}
