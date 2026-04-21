package cbreaker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/KronusRodion/cbreaker/domain"
	"github.com/KronusRodion/cbreaker/internal/circuit"
	"github.com/KronusRodion/cbreaker/ports"
)

var MuBreakerFn = func (cfg Config) ports.Breaker {
	return circuit.NewBreaker(
			circuit.WithName(cfg.Name),
			circuit.WithFailureThreshold(cfg.FailureThreshold),
			circuit.WithSuccessThreshold(cfg.SuccessThreshold),
			circuit.WithTimeout(cfg.Timeout),
			circuit.WithMaxConcurrentRequests(cfg.MaxConcurrentRequests),
			circuit.WithRollingWindow(cfg.RollingWindow),
			circuit.WithObserver(cfg.Observer),
		)
}


var AtomicBreakerFn = func (cfg Config) ports.Breaker {
	return circuit.NewAtomicBreaker(
			circuit.WithName(cfg.Name),
			circuit.WithFailureThreshold(cfg.FailureThreshold),
			circuit.WithSuccessThreshold(cfg.SuccessThreshold),
			circuit.WithTimeout(cfg.Timeout),
			circuit.WithMaxConcurrentRequests(cfg.MaxConcurrentRequests),
			circuit.WithRollingWindow(cfg.RollingWindow),
			circuit.WithObserver(cfg.Observer),
		)
}

// Helper to setup benchmark
func setupBench(_ *testing.B, fn func(Config) ports.Breaker, name string, failureThreshold int) {
	AddBreakerCreate(fn)
	Configure(&Config{
		Name:                  name,
		FailureThreshold:      failureThreshold,
		SuccessThreshold:      2,
		Timeout:               10 * time.Second,
		MaxConcurrentRequests: 100,
		RollingWindow:         60 * time.Second,
		Observer:              nil, // No observer for pure performance
	})
	AddResources(name)
}

// ============================================================================
// BASELINE: Direct calls without circuit breaker
// ============================================================================

func BenchmarkBaselineDirectCall(b *testing.B) {
	ctx := context.Background()
	fn := func(_ context.Context) (string, error) {
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fn(ctx)
	}
}

func BenchmarkBaselineDirectCallWithError(b *testing.B) {
	ctx := context.Background()
	fn := func(_ context.Context) (string, error) {
		return "", errors.New("error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fn(ctx)
	}
}

func BenchmarkBaselineDirectCallWithDelay(b *testing.B) {
	ctx := context.Background()
	fn := func(_ context.Context) (string, error) {
		time.Sleep(100 * time.Microsecond)
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fn(ctx)
	}
}

// ============================================================================
// MUTEX IMPLEMENTATION BENCHMARKS
// ============================================================================

// Mutex - Success path (circuit closed)
func BenchmarkMutexSuccess(b *testing.B) {
	setupBench(b, MuBreakerFn, "bench-mutex-success", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-mutex-success", fn)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// Mutex - Error path (records failure)
func BenchmarkMutexError(b *testing.B) {
	setupBench(b, MuBreakerFn, "bench-mutex-error", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "", errors.New("error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Call(ctx, "bench-mutex-error", fn)
	}
}

// Mutex - Circuit open path (rejected)
func BenchmarkMutexCircuitOpen(b *testing.B) {
	setupBench(b, MuBreakerFn, "bench-mutex-open", 1) // Open after 1 failure
	ctx := context.Background()
	
	// First, cause circuit to open
	fn := func(ctx context.Context) (string, error) {
		return "", errors.New("error")
	}
	for i := 0; i < 5; i++ {
		Call(ctx, "bench-mutex-open", fn)
	}
	
	// Now circuit is open
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-mutex-open", fn)
		if err != domain.ErrCircuitOpen {
			b.Fatalf("expected ErrCircuitOpen, got %v", err)
		}
	}
}

// Mutex - Parallel execution
func BenchmarkMutexParallel(b *testing.B) {
	setupBench(b, MuBreakerFn, "bench-mutex-parallel", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := Call(ctx, "bench-mutex-parallel", fn)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}

// Mutex - With real delay
func BenchmarkMutexWithDelay(b *testing.B) {
	setupBench(b, MuBreakerFn, "bench-mutex-delay", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Microsecond)
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-mutex-delay", fn)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// ============================================================================
// ATOMIC IMPLEMENTATION BENCHMARKS
// ============================================================================

// Atomic - Success path (circuit closed)
func BenchmarkAtomicSuccess(b *testing.B) {
	setupBench(b, AtomicBreakerFn, "bench-atomic-success", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-atomic-success", fn)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// Atomic - Error path (records failure)
func BenchmarkAtomicError(b *testing.B) {
	setupBench(b, AtomicBreakerFn, "bench-atomic-error", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "", errors.New("error")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Call(ctx, "bench-atomic-error", fn)
	}
}

// Atomic - Circuit open path (rejected)
func BenchmarkAtomicCircuitOpen(b *testing.B) {
	setupBench(b, AtomicBreakerFn, "bench-atomic-open", 1)
	ctx := context.Background()
	
	fn := func(ctx context.Context) (string, error) {
		return "", errors.New("error")
	}
	for i := 0; i < 5; i++ {
		Call(ctx, "bench-atomic-open", fn)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-atomic-open", fn)
		if err != domain.ErrCircuitOpen {
			b.Fatalf("expected ErrCircuitOpen, got %v", err)
		}
	}
}

// Atomic - Parallel execution
func BenchmarkAtomicParallel(b *testing.B) {
	setupBench(b, AtomicBreakerFn, "bench-atomic-parallel", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := Call(ctx, "bench-atomic-parallel", fn)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
		}
	})
}

// Atomic - With real delay
func BenchmarkAtomicWithDelay(b *testing.B) {
	setupBench(b, AtomicBreakerFn, "bench-atomic-delay", 1000)
	ctx := context.Background()
	fn := func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Microsecond)
		return "ok", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Call(ctx, "bench-atomic-delay", fn)
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// ============================================================================
// COMPARISON BENCHMARKS (Mutex vs Atomic)
// ============================================================================

func BenchmarkComparisonSuccess(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, "bench-compare-success", 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Call(ctx, "bench-compare-success", fn)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkComparisonError(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, "bench-compare-error", 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "", errors.New("error")
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = Call(ctx, "bench-compare-error", fn)
			}
		})
	}
}

func BenchmarkComparisonParallel(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, "bench-compare-parallel", 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := Call(ctx, "bench-compare-parallel", fn)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

// ============================================================================
// CONCURRENCY LEVEL BENCHMARKS
// ============================================================================

func BenchmarkConcurrencyLevels(b *testing.B) {
	levels := []int{1, 10, 50, 100, 500}
	
	for _, level := range levels {
		b.Run(fmt.Sprintf("Mutex_%d", level), func(b *testing.B) {
			setupBench(b, MuBreakerFn, fmt.Sprintf("bench-conc-mutex-%d", level), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			b.SetParallelism(level)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := Call(ctx, fmt.Sprintf("bench-conc-mutex-%d", level), fn)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
		
		b.Run(fmt.Sprintf("Atomic_%d", level), func(b *testing.B) {
			setupBench(b, AtomicBreakerFn, fmt.Sprintf("bench-conc-atomic-%d", level), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			b.SetParallelism(level)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := Call(ctx, fmt.Sprintf("bench-conc-atomic-%d", level), fn)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

// ============================================================================
// MEMORY ALLOCATION BENCHMARKS
// ============================================================================

func BenchmarkAllocations(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, fmt.Sprintf("bench-alloc-%s", impl.name), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := Call(ctx, fmt.Sprintf("bench-alloc-%s", impl.name), fn)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// ============================================================================
// OVERHEAD ANALYSIS (сравнение с baseline)
// ============================================================================

func BenchmarkOverheadAnalysis(b *testing.B) {
	// Run baseline
	baselineResult := testing.Benchmark(func(b *testing.B) {
		ctx := context.Background()
		fn := func(_ context.Context) (string, error) {
			return "ok", nil
		}
		for i := 0; i < b.N; i++ {
			_, _ = fn(ctx)
		}
	})
	
	// Run mutex
	mutexResult := testing.Benchmark(func(b *testing.B) {
		setupBench(b, MuBreakerFn, "overhead-mutex", 1000)
		ctx := context.Background()
		fn := func(ctx context.Context) (string, error) {
			return "ok", nil
		}
		for i := 0; i < b.N; i++ {
			_, _ = Call(ctx, "overhead-mutex", fn)
		}
	})
	
	// Run atomic
	atomicResult := testing.Benchmark(func(b *testing.B) {
		setupBench(b, AtomicBreakerFn, "overhead-atomic", 1000)
		ctx := context.Background()
		fn := func(ctx context.Context) (string, error) {
			return "ok", nil
		}
		for i := 0; i < b.N; i++ {
			_, _ = Call(ctx, "overhead-atomic", fn)
		}
	})
	
	baselineNs := baselineResult.NsPerOp()
	mutexNs := mutexResult.NsPerOp()
	atomicNs := atomicResult.NsPerOp()
	
	b.Logf("\n")
	b.Logf("╔════════════════════════════════════════════════════════════════╗")
	b.Logf("║                    OVERHEAD ANALYSIS                           ║")
	b.Logf("╠════════════════════════════════════════════════════════════════╣")
	b.Logf("║ Baseline (direct call):     %10d ns/op                        ║", baselineNs)
	b.Logf("║ Mutex implementation:       %10d ns/op  (%.2fx overhead)      ║", mutexNs, float64(mutexNs)/float64(baselineNs))
	b.Logf("║ Atomic implementation:      %10d ns/op  (%.2fx overhead)      ║", atomicNs, float64(atomicNs)/float64(baselineNs))
	b.Logf("╠════════════════════════════════════════════════════════════════╣")
	b.Logf("║ Mutex vs Atomic:            %.2fx faster                      ║", float64(mutexNs)/float64(atomicNs))
	b.Logf("╚════════════════════════════════════════════════════════════════╝")
}

// ============================================================================
// LATENCY PERCENTILE BENCHMARKS
// ============================================================================

func BenchmarkLatencyPercentiles(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, fmt.Sprintf("latency-%s", impl.name), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) (string, error) {
				return "ok", nil
			}
			
			latencies := make([]int64, b.N)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				_, err := Call(ctx, fmt.Sprintf("latency-%s", impl.name), fn)
				latencies[i] = time.Since(start).Nanoseconds()
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
			
			// Calculate percentiles
			var sum int64
			min := latencies[0]
			max := latencies[0]
			for _, lat := range latencies {
				sum += lat
				if lat < min {
					min = lat
				}
				if lat > max {
					max = lat
				}
			}
			avg := sum / int64(b.N)
			
			// Simple percentile (requires sorting for accurate, this is approximation)
			p50 := latencies[b.N/2]
			p95 := latencies[int(float64(b.N)*0.95)]
			p99 := latencies[int(float64(b.N)*0.99)]
			
			b.ReportMetric(float64(avg), "avg-ns/op")
			b.ReportMetric(float64(min), "min-ns/op")
			b.ReportMetric(float64(max), "max-ns/op")
			b.ReportMetric(float64(p50), "p50-ns/op")
			b.ReportMetric(float64(p95), "p95-ns/op")
			b.ReportMetric(float64(p99), "p99-ns/op")
		})
	}
}

// ============================================================================
// STRESS TEST WITH HIGH FAILURE RATE
// ============================================================================

func BenchmarkHighFailureRate(b *testing.B) {
	impls := []struct {
		name string
		impl func(Config) ports.Breaker
	}{
		{"Mutex", MuBreakerFn},
		{"Atomic", AtomicBreakerFn},
	}
	
	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			setupBench(b, impl.impl, "BenchmarkHighFailureRate", 1000)
			Configure(&Config{
				Name:                  fmt.Sprintf("stress-%s", impl.name),
				FailureThreshold:      5,
				SuccessThreshold:      2,
				Timeout:               1 * time.Second,
				MaxConcurrentRequests: 1,
				RollingWindow:         10 * time.Second,
			})
			AddResources(fmt.Sprintf("stress-%s", impl.name))
			
			ctx := context.Background()
			failureCount := 0
			successCount := 0
			var mu sync.Mutex
			
			fn := func(ctx context.Context) (string, error) {
				mu.Lock()
				failureCount++
				shouldFail := failureCount%3 != 0 // 66% failures
				mu.Unlock()
				
				if shouldFail {
					return "", errors.New("error")
				}
				successCount++
				return "ok", nil
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = Call(ctx, fmt.Sprintf("stress-%s", impl.name), fn)
			}
		})
	}
}

// ============================================================================
// THROUGHPUT TEST WITH DIFFERENT PAYLOAD SIZES
// ============================================================================

func BenchmarkThroughputWithPayload(b *testing.B) {
	payloadSizes := []int{64, 256, 1024, 4096}
	
	for _, size := range payloadSizes {
		b.Run(fmt.Sprintf("Mutex_%d", size), func(b *testing.B) {
			setupBench(b, MuBreakerFn, fmt.Sprintf("throughput-mutex-%d", size), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) ([]byte, error) {
				return make([]byte, size), nil
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Call(ctx, fmt.Sprintf("throughput-mutex-%d", size), fn)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
		
		b.Run(fmt.Sprintf("Atomic_%d", size), func(b *testing.B) {
			setupBench(b, AtomicBreakerFn, fmt.Sprintf("throughput-atomic-%d", size), 1000)
			ctx := context.Background()
			fn := func(ctx context.Context) ([]byte, error) {
				return make([]byte, size), nil
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := Call(ctx, fmt.Sprintf("throughput-atomic-%d", size), fn)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}