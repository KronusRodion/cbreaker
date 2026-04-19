package window

import (
	"sync"
	"testing"
	"time"
)

// TestNewSlidingWindow tests the creation of sliding window
func TestNewSlidingWindow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		windowSize  time.Duration
		bucketSize  time.Duration
		wantBuckets int
	}{
		{
			name:        "standard window with 60 buckets",
			windowSize:  60 * time.Second,
			bucketSize:  1 * time.Second,
			wantBuckets: 60,
		},
		{
			name:        "window with 10 buckets",
			windowSize:  10 * time.Second,
			bucketSize:  1 * time.Second,
			wantBuckets: 10,
		},
		{
			name:        "window equals bucket size",
			windowSize:  5 * time.Second,
			bucketSize:  5 * time.Second,
			wantBuckets: 1,
		},
		{
			name:        "window smaller than bucket size",
			windowSize:  1 * time.Second,
			bucketSize:  5 * time.Second,
			wantBuckets: 1,
		},
		{
			name:        "large window",
			windowSize:  300 * time.Second,
			bucketSize:  10 * time.Second,
			wantBuckets: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := NewSlidingWindow(tt.windowSize, tt.bucketSize)
			
			if len(sw.buckets) != tt.wantBuckets {
				t.Errorf("Expected %d buckets, got %d", tt.wantBuckets, len(sw.buckets))
			}
			
			if sw.windowSize != tt.windowSize {
				t.Errorf("Expected windowSize %v, got %v", tt.windowSize, sw.windowSize)
			}
			
			if sw.bucketSize != tt.bucketSize {
				t.Errorf("Expected bucketSize %v, got %v", tt.bucketSize, sw.bucketSize)
			}
			
			if sw.totalFailures != 0 {
				t.Errorf("Expected totalFailures 0, got %d", sw.totalFailures)
			}
			
			if sw.totalSuccesses != 0 {
				t.Errorf("Expected totalSuccesses 0, got %d", sw.totalSuccesses)
			}
			
			if sw.consecutive != 0 {
				t.Errorf("Expected consecutive 0, got %d", sw.consecutive)
			}
		})
	}
}

// TestRecordFailure tests recording failures
func TestRecordFailure(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Record first failure
	sw.RecordFailure()
	
	if sw.totalFailures != 1 {
		t.Errorf("Expected totalFailures 1, got %d", sw.totalFailures)
	}
	
	if sw.consecutive != 0 {
		t.Errorf("Expected consecutive 0 after failure, got %d", sw.consecutive)
	}
	
	// Record second failure
	sw.RecordFailure()
	
	if sw.totalFailures != 2 {
		t.Errorf("Expected totalFailures 2, got %d", sw.totalFailures)
	}
	
	// Check bucket
	if sw.buckets[sw.currentIdx].failures != 2 {
		t.Errorf("Expected bucket failures 2, got %d", sw.buckets[sw.currentIdx].failures)
	}
}

// TestRecordSuccess tests recording successes
func TestRecordSuccess(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Record first success
	sw.RecordSuccess()
	
	if sw.totalSuccesses != 1 {
		t.Errorf("Expected totalSuccesses 1, got %d", sw.totalSuccesses)
	}
	
	if sw.consecutive != 1 {
		t.Errorf("Expected consecutive 1, got %d", sw.consecutive)
	}
	
	// Record second success
	sw.RecordSuccess()
	
	if sw.totalSuccesses != 2 {
		t.Errorf("Expected totalSuccesses 2, got %d", sw.totalSuccesses)
	}
	
	if sw.consecutive != 2 {
		t.Errorf("Expected consecutive 2, got %d", sw.consecutive)
	}
	
	// Check bucket
	if sw.buckets[sw.currentIdx].successes != 2 {
		t.Errorf("Expected bucket successes 2, got %d", sw.buckets[sw.currentIdx].successes)
	}
}

// TestMixedRecords tests interleaved successes and failures
func TestMixedRecords(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Pattern: success, success, failure, success, failure, failure
	sw.RecordSuccess() // consecutive: 1
	sw.RecordSuccess() // consecutive: 2
	sw.RecordFailure() // consecutive: 0
	sw.RecordSuccess() // consecutive: 1
	sw.RecordFailure() // consecutive: 0
	sw.RecordFailure() // consecutive: 0
	
	if sw.totalSuccesses != 3 {
		t.Errorf("Expected totalSuccesses 3, got %d", sw.totalSuccesses)
	}
	
	if sw.totalFailures != 3 {
		t.Errorf("Expected totalFailures 3, got %d", sw.totalFailures)
	}
	
	if sw.consecutive != 0 {
		t.Errorf("Expected consecutive 0, got %d", sw.consecutive)
	}
}

// TestFailureRate tests failure rate calculation
func TestFailureRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		setup         func(*SlidingWindow)
		expectedRate  float64
	}{
		{
			name: "empty window",
			setup: func(sw *SlidingWindow) {
				// Do nothing
			},
			expectedRate: 0.0,
		},
		{
			name: "all successes",
			setup: func(sw *SlidingWindow) {
				for i := 0; i < 10; i++ {
					sw.RecordSuccess()
				}
			},
			expectedRate: 0.0,
		},
		{
			name: "all failures",
			setup: func(sw *SlidingWindow) {
				for i := 0; i < 10; i++ {
					sw.RecordFailure()
				}
			},
			expectedRate: 1.0,
		},
		{
			name: "mixed 50% failures",
			setup: func(sw *SlidingWindow) {
				for i := 0; i < 5; i++ {
					sw.RecordSuccess()
				}
				for i := 0; i < 5; i++ {
					sw.RecordFailure()
				}
			},
			expectedRate: 0.5,
		},
		{
			name: "mixed 30% failures",
			setup: func(sw *SlidingWindow) {
				for i := 0; i < 7; i++ {
					sw.RecordSuccess()
				}
				for i := 0; i < 3; i++ {
					sw.RecordFailure()
				}
			},
			expectedRate: 0.3,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sw := NewSlidingWindow(10*time.Second, 1*time.Second)
			tt.setup(sw)
			
			rate := sw.FailureRate()
			
			if rate != tt.expectedRate {
				t.Errorf("Expected failure rate %f, got %f", tt.expectedRate, rate)
			}
		})
	}
}

// TestConsecutiveSuccesses tests consecutive success tracking
func TestConsecutiveSuccesses(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Record 3 successes
	for i := 0; i < 3; i++ {
		sw.RecordSuccess()
	}
	
	if sw.ConsecutiveSuccesses() != 3 {
		t.Errorf("Expected consecutive successes 3, got %d", sw.ConsecutiveSuccesses())
	}
	
	// A failure resets consecutive counter
	sw.RecordFailure()
	
	if sw.ConsecutiveSuccesses() != 0 {
		t.Errorf("Expected consecutive successes 0 after failure, got %d", sw.ConsecutiveSuccesses())
	}
	
	// Success after failure starts new streak
	sw.RecordSuccess()
	
	if sw.ConsecutiveSuccesses() != 1 {
		t.Errorf("Expected consecutive successes 1, got %d", sw.ConsecutiveSuccesses())
	}
}

// TestReset tests resetting the sliding window
func TestReset(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Add some data
	sw.RecordSuccess()
	sw.RecordSuccess()
	sw.RecordFailure()
	sw.RecordFailure()
	
	// Verify data was added
	if sw.totalSuccesses != 2 {
		t.Errorf("Expected totalSuccesses 2 before reset, got %d", sw.totalSuccesses)
	}
	if sw.totalFailures != 2 {
		t.Errorf("Expected totalFailures 2 before reset, got %d", sw.totalFailures)
	}
	if sw.consecutive != 0 {
		t.Errorf("Expected consecutive 0 before reset, got %d", sw.consecutive)
	}
	
	// Reset
	sw.Reset()
	
	// Verify reset
	if sw.totalSuccesses != 0 {
		t.Errorf("Expected totalSuccesses 0 after reset, got %d", sw.totalSuccesses)
	}
	if sw.totalFailures != 0 {
		t.Errorf("Expected totalFailures 0 after reset, got %d", sw.totalFailures)
	}
	if sw.consecutive != 0 {
		t.Errorf("Expected consecutive 0 after reset, got %d", sw.consecutive)
	}
	if sw.currentIdx != 0 {
		t.Errorf("Expected currentIdx 0 after reset, got %d", sw.currentIdx)
	}
	
	// All buckets should be nil
	for i, bucket := range sw.buckets {
		if bucket != nil {
			t.Errorf("Bucket %d should be nil after reset, got %v", i, bucket)
		}
	}
}

// TestTotalFailures tests total failures counter
func TestTotalFailures(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	if sw.TotalFailures() != 0 {
		t.Errorf("Expected total failures 0, got %d", sw.TotalFailures())
	}
	
	sw.RecordFailure()
	if sw.TotalFailures() != 1 {
		t.Errorf("Expected total failures 1, got %d", sw.TotalFailures())
	}
	
	sw.RecordFailure()
	sw.RecordFailure()
	if sw.TotalFailures() != 3 {
		t.Errorf("Expected total failures 3, got %d", sw.TotalFailures())
	}
}

// TestConcurrency tests thread safety with multiple goroutines
func TestConcurrency(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100
	
	// Run concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if j%2 == 0 {
					sw.RecordSuccess()
				} else {
					sw.RecordFailure()
				}
			}
		}()
	}
	
	wg.Wait()
	
	// Expected totals
	expectedTotal := numGoroutines * numOperations
	expectedSuccesses := expectedTotal / 2
	expectedFailures := expectedTotal / 2
	
	if sw.totalSuccesses != expectedSuccesses {
		t.Errorf("Expected totalSuccesses %d, got %d", expectedSuccesses, sw.totalSuccesses)
	}
	
	if sw.totalFailures != expectedFailures {
		t.Errorf("Expected totalFailures %d, got %d", expectedFailures, sw.totalFailures)
	}
	
	// Failure rate should be 0.5
	rate := sw.FailureRate()
	if rate != 0.5 {
		t.Errorf("Expected failure rate 0.5, got %f", rate)
	}
}

// TestWindowExpiration tests that old records expire
func TestWindowExpiration(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(2*time.Second, 1*time.Second)
	
	// Record failures
	sw.RecordFailure()
	sw.RecordFailure()
	
	if sw.TotalFailures() != 2 {
		t.Errorf("Expected 2 failures, got %d", sw.TotalFailures())
	}
	
	// Wait for window to expire
	time.Sleep(3 * time.Second)
	
	// Record a success - this should trigger advance and expire old buckets
	sw.RecordSuccess()
	
	// Old failures should be expired, only the new success should remain
	if sw.TotalFailures() != 0 {
		t.Errorf("Expected failures to expire, got %d", sw.TotalFailures())
	}
	
	if sw.totalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", sw.totalSuccesses)
	}
}

// TestPartialWindowExpiration tests that only old records expire
func TestPartialWindowExpiration(t *testing.T) {
	t.Parallel()
	sw := NewSlidingWindow(3*time.Second, 1*time.Second)
	
	// Record first batch
	sw.RecordFailure()
	sw.RecordFailure()
	
	time.Sleep(2 * time.Second)
	
	// Record second batch
	sw.RecordFailure()
	sw.RecordSuccess()
	
	// Wait for first batch to expire but second batch should remain
	time.Sleep(2 * time.Second)
	
	// Record another to trigger advance
	sw.RecordSuccess()
	
	// First batch (2 failures) should be expired
	// Second batch (1 failure, 1 success) should remain
	// New success should be added
	if sw.TotalFailures() != 1 {
		t.Errorf("Expected 1 failure remaining, got %d", sw.TotalFailures())
	}
	
	if sw.totalSuccesses != 2 {
		t.Errorf("Expected 2 successes, got %d", sw.totalSuccesses)
	}
}

// TestBucketRotation tests that buckets rotate correctly
func TestBucketRotation(t *testing.T) {
	sw := NewSlidingWindow(3*time.Second, 1*time.Second)
	
	// Record in first bucket
	sw.RecordFailure()
	initialIdx := sw.currentIdx
	
	// Wait to move to next bucket
	time.Sleep(1100 * time.Millisecond)
	
	// Record in second bucket
	sw.RecordSuccess()
	
	if sw.currentIdx == initialIdx {
		t.Errorf("Expected bucket index to change, but it stayed at %d", sw.currentIdx)
	}
	
	// Both records should be counted
	if sw.TotalFailures() != 1 {
		t.Errorf("Expected 1 failure, got %d", sw.TotalFailures())
	}
	
	if sw.totalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", sw.totalSuccesses)
	}
}

// TestEmptyBucketHandling tests handling of empty buckets
func TestEmptyBucketHandling(t *testing.T) {
	sw := NewSlidingWindow(5*time.Second, 1*time.Second)
	
	// Record some data
	sw.RecordSuccess()
	sw.RecordFailure()
	
	// Wait for multiple bucket shifts
	time.Sleep(6 * time.Second)
	
	// Record new data - this should clean up old buckets
	sw.RecordSuccess()
	
	// Only the new success should remain
	if sw.TotalFailures() != 0 {
		t.Errorf("Expected 0 failures after expiration, got %d", sw.TotalFailures())
	}
	
	if sw.totalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", sw.totalSuccesses)
	}
}

// BenchmarkRecordSuccess benchmarks success recording
func BenchmarkRecordSuccess(b *testing.B) {
	sw := NewSlidingWindow(60*time.Second, 1*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.RecordSuccess()
	}
}

// BenchmarkRecordFailure benchmarks failure recording
func BenchmarkRecordFailure(b *testing.B) {
	sw := NewSlidingWindow(60*time.Second, 1*time.Second)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.RecordFailure()
	}
}

// BenchmarkFailureRate benchmarks failure rate calculation
func BenchmarkFailureRate(b *testing.B) {
	sw := NewSlidingWindow(60*time.Second, 1*time.Second)
	
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			sw.RecordSuccess()
		} else {
			sw.RecordFailure()
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.FailureRate()
	}
}

// BenchmarkConcurrentRecords benchmarks concurrent recording
func BenchmarkConcurrentRecords(b *testing.B) {
	sw := NewSlidingWindow(60*time.Second, 1*time.Second)
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sw.RecordSuccess()
		}
	})
}

// TestLargeNumberOfRecords tests handling of large number of records
func TestLargeNumberOfRecords(t *testing.T) {
	sw := NewSlidingWindow(60*time.Second, 1*time.Second)
	
	// Record large number of operations
	for i := 0; i < 10000; i++ {
		if i%3 == 0 {
			sw.RecordFailure()
		} else {
			sw.RecordSuccess()
		}
	}
	
	// Verify counts
	expectedSuccesses := 6666 // Approximately 2/3 of 10000
	expectedFailures := 3334  // Approximately 1/3 of 10000
	
	if sw.totalSuccesses < expectedSuccesses-100 || sw.totalSuccesses > expectedSuccesses+100 {
		t.Errorf("Expected successes around %d, got %d", expectedSuccesses, sw.totalSuccesses)
	}
	
	if sw.totalFailures < expectedFailures-100 || sw.totalFailures > expectedFailures+100 {
		t.Errorf("Expected failures around %d, got %d", expectedFailures, sw.totalFailures)
	}
	
	// Failure rate should be approximately 0.333
	rate := sw.FailureRate()
	if rate < 0.33 || rate > 0.34 {
		t.Errorf("Expected failure rate around 0.333, got %f", rate)
	}
}

// TestZeroWindowSize tests edge case with zero window size
func TestZeroWindowSize(t *testing.T) {
	sw := NewSlidingWindow(0, 1*time.Second)
	
	// Should create at least 1 bucket
	if len(sw.buckets) < 1 {
		t.Errorf("Expected at least 1 bucket, got %d", len(sw.buckets))
	}
	
	// Should still work
	sw.RecordSuccess()
	if sw.totalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", sw.totalSuccesses)
	}
}

// TestAddMethod tests the Add method (currently a placeholder)
func TestAddMethod(t *testing.T) {
	sw := NewSlidingWindow(10*time.Second, 1*time.Second)
	
	// Add should not panic
	sw.Add(100 * time.Millisecond)
	sw.Add(200 * time.Millisecond)
	sw.Add(300 * time.Millisecond)
	
	// State should remain unchanged
	if sw.totalSuccesses != 0 {
		t.Errorf("Expected totalSuccesses 0, got %d", sw.totalSuccesses)
	}
}