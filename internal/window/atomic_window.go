package window

import (
	"sync/atomic"
	"time"
	"unsafe"
)

// atomicBucket represents a time bucket with atomic operations
type atomicBucket struct {
	failures  atomic.Int32
	successes atomic.Int32
	timestamp atomic.Int64 // Unix timestamp
}

// AtomicSlidingWindow implements lock-free sliding window
type AtomicSlidingWindow struct {
	windowSize  time.Duration
	bucketSize  time.Duration
	buckets     []*atomicBucket
	bucketsPtr  unsafe.Pointer // atomic pointer to buckets slice
	currentIdx  atomic.Int32
	consecutive atomic.Int32
	
	totalFailures  atomic.Int64
	totalSuccesses atomic.Int64
}

// NewAtomicSlidingWindow creates a lock-free sliding window
func NewAtomicSlidingWindow(windowSize time.Duration, bucketSize time.Duration) *AtomicSlidingWindow {
	numBuckets := int(windowSize / bucketSize)
	if numBuckets < 1 {
		numBuckets = 1
	}

	buckets := make([]*atomicBucket, numBuckets)
	for i := range buckets {
		buckets[i] = &atomicBucket{}
	}

	sw := &AtomicSlidingWindow{
		windowSize: windowSize,
		bucketSize: bucketSize,
		buckets:    buckets,
	}
	atomic.StorePointer(&sw.bucketsPtr, unsafe.Pointer(&buckets))
	
	return sw
}

// advance moves the window forward if needed (lock-free)
func (sw *AtomicSlidingWindow) advance() {
	now := time.Now().UnixNano()
	bucketSizeNano := sw.bucketSize.Nanoseconds()
	
	// Get current bucket index
	idx := sw.currentIdx.Load()
	bucketsPtr := (*[]*atomicBucket)(atomic.LoadPointer(&sw.bucketsPtr))
	buckets := *bucketsPtr
	
	currentBucket := buckets[idx]
	timestamp := currentBucket.timestamp.Load()
	
	if timestamp == 0 {
		// Initialize bucket
		currentBucket.timestamp.CompareAndSwap(0, now)
		return
	}
	
	timeDiff := now - timestamp
	bucketShift := int(timeDiff / bucketSizeNano)
	
	if bucketShift <= 0 {
		return
	}
	
	// Limit shift to number of buckets
	if bucketShift > len(buckets) {
		bucketShift = len(buckets)
	}
	
	// Shift buckets atomically
	for i := 0; i < bucketShift; i++ {
		newIdx := (sw.currentIdx.Load() + 1) % int32(len(buckets))
		
		// Get old bucket before moving
		oldBucket := buckets[newIdx]
		
		// Subtract old bucket from totals
		if oldFailures := oldBucket.failures.Swap(0); oldFailures > 0 {
			sw.totalFailures.Add(-int64(oldFailures))
		}
		if oldSuccesses := oldBucket.successes.Swap(0); oldSuccesses > 0 {
			sw.totalSuccesses.Add(-int64(oldSuccesses))
		}
		
		// Reset timestamp
		oldBucket.timestamp.Store(now)
		
		// Move to next bucket
		sw.currentIdx.Store(newIdx)
	}
}

// RecordFailure records a failure (lock-free)
func (sw *AtomicSlidingWindow) RecordFailure() {
	sw.advance()
	
	bucketsPtr := (*[]*atomicBucket)(atomic.LoadPointer(&sw.bucketsPtr))
	buckets := *bucketsPtr
	idx := sw.currentIdx.Load()
	
	buckets[idx].failures.Add(1)
	sw.totalFailures.Add(1)
	sw.consecutive.Store(0)
}

// RecordSuccess records a success (lock-free)
func (sw *AtomicSlidingWindow) RecordSuccess() {
	sw.advance()
	
	bucketsPtr := (*[]*atomicBucket)(atomic.LoadPointer(&sw.bucketsPtr))
	buckets := *bucketsPtr
	idx := sw.currentIdx.Load()
	
	buckets[idx].successes.Add(1)
	sw.totalSuccesses.Add(1)
	sw.consecutive.Add(1)
}

// FailureRate returns the current failure rate (lock-free read)
func (sw *AtomicSlidingWindow) FailureRate() float64 {
	totalFailures := sw.totalFailures.Load()
	totalSuccesses := sw.totalSuccesses.Load()
	total := totalFailures + totalSuccesses
	
	if total == 0 {
		return 0.0
	}
	return float64(totalFailures) / float64(total)
}

// TotalFailures returns total failures (lock-free read)
func (sw *AtomicSlidingWindow) TotalFailures() int {
	return int(sw.totalFailures.Load())
}

// ConsecutiveSuccesses returns consecutive successes (lock-free read)
func (sw *AtomicSlidingWindow) ConsecutiveSuccesses() int {
	return int(sw.consecutive.Load())
}

// Reset resets the window (lock-free)
func (sw *AtomicSlidingWindow) Reset() {
	bucketsPtr := (*[]*atomicBucket)(atomic.LoadPointer(&sw.bucketsPtr))
	buckets := *bucketsPtr
	
	// Reset all buckets
	for i := range buckets {
		buckets[i].failures.Store(0)
		buckets[i].successes.Store(0)
		buckets[i].timestamp.Store(0)
	}
	
	sw.currentIdx.Store(0)
	sw.consecutive.Store(0)
	sw.totalFailures.Store(0)
	sw.totalSuccesses.Store(0)
}

// Add adds duration for latency tracking (placeholder)
func (sw *AtomicSlidingWindow) Add(duration time.Duration) {
	// Can be extended for latency percentiles
	_ = duration
}