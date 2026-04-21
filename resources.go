package cbreaker

import (
	"time"

	"github.com/KronusRodion/cbreaker/internal/circuit"
	"github.com/KronusRodion/cbreaker/ports"
)

var breakers map[string]ports.Breaker = make(map[string]ports.Breaker)
var createBreaker func(cfg Config) ports.Breaker
var globalCfg Config

// Init инициализирует глобальный конфиг и функцию создания cbreaker
func init() {
	globalCfg = DefaultConfig()

	createBreaker = func(cfg Config) ports.Breaker {

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
}

// AddBreakerCreate - adds a function that will be used
// to create a cbreaker when adding resources via AddResources
func AddBreakerCreate(fn func(cfg Config) ports.Breaker) {
	createBreaker = fn
}

// AddResources - added resources to global scope
// so you can use Call with them
func AddResources(resources ...string) {
	for _, v := range resources {
		breakers[v] = createBreaker(globalCfg)
	}
}

// DefaultConfig returns default config with no observers
func DefaultConfig() Config {

	return Config{
		Name:                  "default-cbreaker",
		FailureThreshold:      5,
		SuccessThreshold:      2,
		Timeout:               5 * time.Second,
		MaxConcurrentRequests: 1,
		RollingWindow:         60 * time.Second,
		Observer:              nil,
	}
}

// CurrentConfig возвращает текущий зарегестрированный конфиг
func CurrentConfig() Config {
	return globalCfg
}

// Configure update global config, that uses via createBreaker func
func Configure(cfg *Config) {
	if cfg == nil {
		panic("cfg is nil")
	}
	globalCfg = *cfg
}
