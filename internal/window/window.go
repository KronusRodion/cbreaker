package window

import (
    "sync"
    "time"
)

type bucket struct {
    failures  int
    successes int
    timestamp time.Time
}

type SlidingWindow struct {
    mu          sync.RWMutex
    windowSize  time.Duration
    bucketSize  time.Duration
    buckets     []*bucket
    currentIdx  int
    consecutive int
    totalFailures int
    totalSuccesses int
}

func NewSlidingWindow(windowSize time.Duration, bucketSize time.Duration) *SlidingWindow {
    numBuckets := max(int(windowSize / bucketSize), 1)
    
    return &SlidingWindow{
        windowSize: windowSize,
        bucketSize: bucketSize,
        buckets:    make([]*bucket, numBuckets),
    }
}

func (sw *SlidingWindow) advance() {
    now := time.Now()
    lastBucket := sw.buckets[sw.currentIdx]
    
    if lastBucket == nil {
        sw.buckets[sw.currentIdx] = &bucket{timestamp: now}
        return
    }
    
    timeDiff := now.Sub(lastBucket.timestamp)
    bucketShift := int(timeDiff / sw.bucketSize)
    
    if bucketShift <= 0 {
        return
    }
    
    // Shift buckets
    for i := 0; i < bucketShift && i < len(sw.buckets); i++ {
        sw.currentIdx = (sw.currentIdx + 1) % len(sw.buckets)
        
        // Subtract old bucket from totals
        oldBucket := sw.buckets[sw.currentIdx]
        if oldBucket != nil {
            sw.totalFailures -= oldBucket.failures
            sw.totalSuccesses -= oldBucket.successes
        }
        
        // Reset bucket
        sw.buckets[sw.currentIdx] = &bucket{timestamp: now}
    }
}

func (sw *SlidingWindow) RecordFailure() {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    sw.advance()
    sw.buckets[sw.currentIdx].failures++
    sw.totalFailures++
    sw.consecutive = 0
}

func (sw *SlidingWindow) RecordSuccess() {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    sw.advance()
    sw.buckets[sw.currentIdx].successes++
    sw.totalSuccesses++
    sw.consecutive++
}

func (sw *SlidingWindow) FailureRate() float64 {
    sw.mu.RLock()
    defer sw.mu.RUnlock()
    
    total := sw.totalFailures + sw.totalSuccesses
    if total == 0 {
        return 0.0
    }
    
    return float64(sw.totalFailures) / float64(total)
}

func (sw *SlidingWindow) TotalFailures() int {
    sw.mu.RLock()
    defer sw.mu.RUnlock()
    return sw.totalFailures
}

func (sw *SlidingWindow) ConsecutiveSuccesses() int {
    sw.mu.RLock()
    defer sw.mu.RUnlock()
    return sw.consecutive
}

func (sw *SlidingWindow) Reset() {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    sw.buckets = make([]*bucket, len(sw.buckets))
    sw.currentIdx = 0
    sw.consecutive = 0
    sw.totalFailures = 0
    sw.totalSuccesses = 0
}

func (sw *SlidingWindow) Add(duration time.Duration) {
    // Can be extended to track latency percentiles
    _ = duration
}