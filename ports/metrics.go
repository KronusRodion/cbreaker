package ports

// Metrics interface
type Metrics interface {
	TotalRequests() int64
	TotalFailures() int64
	TotalSuccesses() int64
	FailureRate() float64
	ConsecutiveSuccesses() int
}
