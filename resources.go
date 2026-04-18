package cbreaker

import (
	"time"

	"github.com/KronusRodion/cbreaker/circuit"
	"github.com/KronusRodion/cbreaker/ports"
)

var breakers map[string]ports.Breaker = make(map[string]ports.Breaker)
var createBreaker func(cfg Config) ports.Breaker
var globalCfg Config

// Init инициализирует глобальный circuit breaker
func init() {
	globalCfg = DefaultConfig()

	createBreaker = func(cfg Config) ports.Breaker {

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
}

func AddBreakerCreate(fn func(cfg Config) ports.Breaker) {
	createBreaker = fn
}

func AddResources(resources ...string) {
	for _, v := range resources {
		breakers[v] = createBreaker(globalCfg)
	}
}

// DefaultConfig возвращает конфиг по умолчанию
func DefaultConfig() Config {
	return Config{
		Name:                  "default-circuit-breaker",
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

// Configure конфигурирует глобальный circuit breaker
func Configure(cfg *Config) {
	if cfg == nil {
		panic("cfg is nil")
	}
	globalCfg = *cfg
}
