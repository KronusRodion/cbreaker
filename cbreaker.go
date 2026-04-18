package cbreaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/ports"
)

var (
	mu sync.RWMutex = sync.RWMutex{}
)

// Config глобальной конфигурации
type Config struct {
	Name                  string
	FailureThreshold      int
	SuccessThreshold      int
	Timeout               time.Duration
	MaxConcurrentRequests int
	RollingWindow         time.Duration
	Observer              ports.Observer
}

// Call выполняет функцию через глобальный circuit breaker конкретного ресурса с типобезопасностью
func Call[T any](ctx context.Context, resourceName string, fn func(context.Context) (T, error)) (T, error) {
	var result T
	mu.RLock()
	breaker, ok := breakers[resourceName]
	mu.RUnlock()

	if !ok {
		panic(fmt.Sprintf("resourceName %s is not added in cbreaker", resourceName))
	}

	// Try to acquire state
	state, err := breaker.TryAcquire(ctx)
	if err != nil {
		breaker.NotifyRejected(ctx)
		return result, err
	}

	// Execute with timeout from context
	start := time.Now()
	result, err = fn(ctx)
	duration := time.Since(start)
	start1 := time.Now()
	// Record outcome
	breaker.RecordOutcome(state, err, duration, start1)

	// Notify observer
	if err != nil {
		go breaker.NotifyFailure(ctx, err)
	} else {
		go breaker.NotifySuccess(ctx, duration)
	}

	return result, err
}

// State возвращает текущее состояние глобального брикера
func GetState(resourceName string) domain.State {
	mu.RLock()
	breaker, ok := breakers[resourceName]
	mu.RUnlock()

	if !ok {
		panic(fmt.Sprintf("resourceName %s is not added in cbreaker", resourceName))
	}

	if breaker == nil {
		return domain.StateClosed
	}
	return breaker.State()
}

// Metrics возвращает все метрики
func GetMetrics() ([]string, []ports.Metrics) {
	metrics := make([]ports.Metrics, 0, len(breakers))
	resources := make([]string, 0, len(breakers))

	for resource, breaker := range breakers {
		metrics = append(metrics, breaker.Metrics())
		resources = append(resources, resource)
	}

	return resources, metrics
}
