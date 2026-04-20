# cbreaker
This is Circuit Breaker repository with safety, simple and fast pattern integration. Project have 70% test covering.
We use unblocking cbreaker state update so request latency rise only about 5ms.

# Fast start
```Go
// Add the resources you want to use cbreaker for
cbreaker.AddResources("postgres", "redis")

// Call your resources func through cbreaker.Call
getUser := func(ctx context.Context) (user, error) {
		return user{name: "Rodni", age: 11}, nil
	}

user, err := cbreaker.Call(ctx, "postgres", getUser)
// If cbreaker will open, u get ErrCircuitOpen from cbreaker/domain
```

# Examples
In examples/ dir you can see MVC, simple and Observers examples

# Configuration
You can configure your cbreaker like that:
```Go
// Configure must be before AddResources!!!
cbreaker.Configure(&cbreaker.Config{
		Name:                  "my-service",
		FailureThreshold:      3,
		SuccessThreshold:      2,
		Timeout:               5 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         30 * time.Second,
		Observer:              nil,
	})

cbreaker.AddResources("postgres")
```


# Observers
If you want to collect metrics or traces of cbreaker you can use package cbreaker/metrics:
```Go
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

```

Also you can notice that you can add your own Observers through multiObserver