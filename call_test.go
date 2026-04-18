package cbreaker

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
)

type user struct {
	name string
	age  int
}

// TestCallWithHighFailureRate тестирует поведение при высоком проценте ошибок
func TestCallWithHighFailureRate(t *testing.T) {
	// Конфигурируем circuit breaker
	Configure(&Config{
		Name:                  "test-service",
		FailureThreshold:      3,
		SuccessThreshold:      2,
		Timeout:               2 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         10 * time.Second,
	})

	AddResources("test-service")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// getUser с 90% ошибок и задержкой 50-100ms
	getUser := func(ctx context.Context) (user, error) {
		// Задержка
		delay := time.Duration(50+time.Now().Nanosecond()%50) * time.Millisecond

		select {
		case <-ctx.Done():
			return user{}, ctx.Err()
		case <-time.After(delay):
		}

		// 90% ошибок
		if time.Now().Nanosecond()%100 < 90 {
			return user{}, errors.New("database connection error")
		}

		return user{name: "TestUser", age: 25}, nil
	}

	var (
		wg            sync.WaitGroup
		successCount  atomic.Int64
		failureCount  atomic.Int64
		rejectedCount atomic.Int64
	)

	requests := 50

	// Запускаем запросы
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := Call(ctx, "test-service", getUser)

			if err != nil {
				if errors.Is(err, domain.ErrCircuitOpen) {
					rejectedCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			} else {
				successCount.Add(1)
			}
		}(i)

		time.Sleep(20 * time.Millisecond)
	}

	wg.Wait()

	// Проверяем, что circuit breaker открылся
	state := GetState("test-service")

	t.Logf("Results: Success=%d, Failure=%d, Rejected=%d",
		successCount.Load(), failureCount.Load(), rejectedCount.Load())
	t.Logf("Final circuit state: %s", state)

	// При 90% ошибок circuit должен открыться
	if state != domain.StateOpen && failureCount.Load() > 10 {
		t.Errorf("Expected circuit to be OPEN, but got %s", state)
	}
}

// TestCallWithLowFailureRate тестирует поведение при низком проценте ошибок
func TestCallWithLowFailureRate(t *testing.T) {
	Configure(&Config{
		Name:                  "stable-service",
		FailureThreshold:      5,
		SuccessThreshold:      2,
		Timeout:               3 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         10 * time.Second,
	})

	AddResources("stable-service")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// getUser с 10% ошибок
	getUser := func(ctx context.Context) (user, error) {
		time.Sleep(10 * time.Millisecond) // небольшая задержка

		// 10% ошибок
		if rand.Intn(100) < 10 {
			return user{}, errors.New("rare error")
		}

		return user{name: "StableUser", age: 30}, nil
	}


	var (
		wg            sync.WaitGroup
		successCount  atomic.Int64
		failureCount  atomic.Int64
		rejectedCount atomic.Int64
	)

	requests := 100

	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := Call(ctx, "stable-service", getUser)

			if err != nil {
				if errors.Is(err, domain.ErrCircuitOpen) {
					rejectedCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			} else {
				successCount.Add(1)
			}
		}()

		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()
	time.Sleep(3 * time.Second)

	state := GetState("stable-service")

	t.Logf("Results: Success=%d, Failure=%d, Rejected=%d",
		successCount.Load(), failureCount.Load(), rejectedCount.Load())
	t.Logf("Final circuit state: %s", state)

	// При 10% ошибок circuit должен остаться закрытым
	if state != domain.StateClosed {
		t.Errorf("Expected circuit to be CLOSED, but got %s", state)
	}
}

// TestCallWithRecovery тестирует восстановление circuit breaker
func TestCallWithRecovery(t *testing.T) {
	Configure(&Config{
		Name:                  "recovery-service",
		FailureThreshold:      3,
		SuccessThreshold:      2,
		Timeout:               2 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         10 * time.Second,
	})

	AddResources("recovery-service")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var (
		wg            sync.WaitGroup
		successCount  atomic.Int64
		failureCount  atomic.Int64
		rejectedCount atomic.Int64
	)

	// Фаза 1: Все запросы падают (15 запросов)
	t.Log("Phase 1: Simulating failures...")
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := Call(ctx, "recovery-service",
				func(ctx context.Context) (user, error) {
					time.Sleep(20 * time.Millisecond)
					return user{}, errors.New("service unavailable")
				})

			if err != nil {
				if errors.Is(err, domain.ErrCircuitOpen) {
					rejectedCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			} else {
				successCount.Add(1)
			}
		}(i)

		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()

	state := GetState("recovery-service")
	t.Logf("After failures - Circuit state: %s", state)

	if state != domain.StateOpen {
		t.Errorf("Circuit should be OPEN after failures, but got %s", state)
	}

	// Ждем, пока circuit перейдет в HalfOpen
	t.Log("Waiting for timeout to transition to HalfOpen...")
	time.Sleep(3 * time.Second)

	// Фаза 2: Запросы начинают успешно выполняться
	t.Log("Phase 2: Service recovered, sending successful requests...")

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := Call(ctx, "recovery-service",
				func(ctx context.Context) (user, error) {
					time.Sleep(20 * time.Millisecond)
					return user{name: "Recovered", age: 25}, nil
				})

			if err != nil {
				if errors.Is(err, domain.ErrCircuitOpen) {
					rejectedCount.Add(1)
				} else {
					failureCount.Add(1)
				}
			} else {
				successCount.Add(1)
			}
		}(i)

		time.Sleep(200 * time.Millisecond)
	}

	wg.Wait()

	state = GetState("recovery-service")
	t.Logf("After recovery - Circuit state: %s", state)
	t.Logf("Final stats - Success: %d, Failures: %d, Rejected: %d",
		successCount.Load(), failureCount.Load(), rejectedCount.Load())

	// После успешных запросов circuit должен закрыться
	if state != domain.StateClosed {
		t.Errorf("Circuit should be CLOSED after recovery, but got %s", state)
	}
}

// TestCallConcurrentRequests тестирует конкурентные запросы
func TestCallConcurrentRequests(t *testing.T) {
	Configure(&Config{
		Name:                  "concurrent-service",
		FailureThreshold:      5,
		SuccessThreshold:      2,
		Timeout:               3 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         10 * time.Second,
	})

	AddResources("concurrent-service")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// getUser с 70% ошибок и случайной задержкой
	getUser := func(ctx context.Context) (user, error) {
		// Случайная задержка от 10 до 100ms
		delay := time.Duration(10+time.Now().Nanosecond()%90) * time.Millisecond

		select {
		case <-ctx.Done():
			return user{}, ctx.Err()
		case <-time.After(delay):
		}

		// 70% ошибок
		if time.Now().Nanosecond()%100 < 70 {
			return user{}, errors.New("concurrent error")
		}

		return user{name: "Concurrent", age: 20}, nil
	}

	var (
		wg            sync.WaitGroup
		successCount  atomic.Int64
		failureCount  atomic.Int64
		rejectedCount atomic.Int64
	)

	requests := 50

	// Запускаем много конкурентных запросов
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			start := time.Now()
			_, err := Call(ctx, "concurrent-service", getUser)
			duration := time.Since(start)

			if err != nil {
				if errors.Is(err, domain.ErrCircuitOpen) {
					rejectedCount.Add(1)
					t.Logf("[%03d] Rejected after %.0fms", id, duration.Seconds()*1000)
				} else {
					failureCount.Add(1)
					t.Logf("[%03d] Failed after %.0fms", id, duration.Seconds()*1000)
				}
			} else {
				successCount.Add(1)
				t.Logf("[%03d] Success after %.0fms", id, duration.Seconds()*1000)
			}
		}(i)
	}

	wg.Wait()

	state := GetState("concurrent-service")

	t.Logf("\n=== Concurrent Test Results ===")
	t.Logf("Total requests: %d", requests)
	t.Logf("Success: %d", successCount.Load())
	t.Logf("Failures: %d", failureCount.Load())
	t.Logf("Rejected: %d", rejectedCount.Load())
	t.Logf("Circuit state: %s", state)

	// Проверяем, что все запросы завершились
	if successCount.Load()+failureCount.Load()+rejectedCount.Load() != int64(requests) {
		t.Errorf("Not all requests completed: %d/%d",
			successCount.Load()+failureCount.Load()+rejectedCount.Load(), requests)
	}
}

// TestCallWithContextCancel тестирует отмену через контекст
func TestCallWithContextCancel(t *testing.T) {
	Configure(&Config{
		Name:             "cancel-service",
		FailureThreshold: 3,
		Timeout:          5 * time.Second,
	})

	AddResources("cancel-service")

	// Создаем контекст, который отменится через 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// getUser с долгой задержкой
	getUser := func(ctx context.Context) (user, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return user{name: "Slow", age: 30}, nil
		case <-ctx.Done():
			return user{}, ctx.Err()
		}
	}

	start := time.Now()
	_, err := Call(ctx, "cancel-service", getUser)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context cancellation, but got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context error, got %v", err)
	}

	if duration > 150*time.Millisecond {
		t.Errorf("Expected fast failure (~100ms), but took %v", duration)
	}

	t.Logf("Context cancelled after %v as expected", duration)
}

// TestCallWithDifferentResources тестирует работу с разными ресурсами
func TestCallWithDifferentResources(t *testing.T) {
	Configure(&Config{
		Name:             "default",
		FailureThreshold: 3,
		Timeout:          2 * time.Second,
	})

	// Добавляем несколько ресурсов
	AddResources("db-master", "db-replica", "cache", "api")

	ctx := context.Background()

	// Разные функции для разных ресурсов
	handlers := map[string]func(context.Context) (string, error){
		"db-master": func(ctx context.Context) (string, error) {
			return "master-data", nil
		},
		"db-replica": func(ctx context.Context) (string, error) {
			return "replica-data", nil
		},
		"cache": func(ctx context.Context) (string, error) {
			return "", errors.New("cache miss")
		},
		"api": func(ctx context.Context) (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "api-response", nil
		},
	}

	var wg sync.WaitGroup

	// Тестируем каждый ресурс
	for resource, handler := range handlers {
		wg.Add(1)
		go func(res string, h func(context.Context) (string, error)) {
			defer wg.Done()

			for i := 0; i < 5; i++ {
				result, err := Call(ctx, res, h)

				if err != nil {
					t.Logf("[%s] Request %d failed: %v", res, i, err)
				} else {
					t.Logf("[%s] Request %d succeeded: %s", res, i, result)
				}

				time.Sleep(100 * time.Millisecond)
			}

			state := GetState(res)
			t.Logf("[%s] Final circuit state: %s", res, state)
		}(resource, handler)
	}

	wg.Wait()

	// Проверяем, что кеш открылся, а остальные закрыты
	cacheState := GetState("cache")
	if cacheState != domain.StateOpen {
		t.Errorf("Cache circuit should be OPEN, but got %s", cacheState)
	}

	for _, res := range []string{"db-master", "db-replica", "api"} {
		state := GetState(res)
		if state != domain.StateClosed {
			t.Errorf("%s circuit should be CLOSED, but got %s", res, state)
		}
	}
}

// BenchmarkCall бенчмарк производительности
func BenchmarkCall(b *testing.B) {
	Configure(&Config{
		Name:             "bench-service",
		FailureThreshold: 100, // Высокий порог, чтобы не открывался
		Timeout:          10 * time.Second,
	})

	AddResources("bench-service")

	ctx := context.Background()

	// Быстрая функция
	fastFunc := func(ctx context.Context) (string, error) {
		return "benchmark", nil
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-service", fastFunc)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// BenchmarkCallWithDelay бенчмарк с задержкой
func BenchmarkCallWithDelay(b *testing.B) {
	Configure(&Config{
		Name:             "delay-service",
		FailureThreshold: 100,
		Timeout:          10 * time.Second,
	})

	AddResources("delay-service")

	ctx := context.Background()

	// Функция с небольшой задержкой
	delayedFunc := func(ctx context.Context) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "delayed", nil
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "delay-service", delayedFunc)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}
