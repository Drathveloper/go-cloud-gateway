package config

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ErrNilConfig is returned when there is no config to validate.
var ErrNilConfig = errors.New("config is nil")

// ErrNegativeDuration is returned when a duration that cannot be negative is.
var ErrNegativeDuration = errors.New("duration must not be negative")

// Validate checks that a gateway can be built from cfg: the required fields are set and
// the values are in range. It applies the rules the readers apply to what they read, so
// that a config built in code is held to the same ones. bootstrap.Initialize calls it.
//
// The rules are the validate tags of the config types plus the ones a tag cannot
// express. Every broken rule is reported, one per line.
//
// The tags are checked with a validator.New(), which is what the readers are given
// unless the application customizes theirs.
func Validate(cfg *Config) error {
	if err := validateConfig(nil, cfg); err != nil {
		return fmt.Errorf("validate config failed: %w", err)
	}
	return nil
}

// validateConfig is the only definition of a valid config: Validate and the readers
// share it. The validator checks the tags, and nil means a validator.New().
func validateConfig(validate *validator.Validate, cfg *Config) error {
	if cfg == nil {
		return ErrNilConfig
	}
	if validate == nil {
		validate = validator.New()
	}
	// Joined as the validator joins the tag errors, so that one run reports everything.
	return errors.Join(validate.Struct(cfg), validateCircuitBreakerDurations(cfg.Gateway.Routes))
}

// validateCircuitBreakerDurations rejects the negative circuit breaker durations. The
// tags cannot: the validator only compares numbers, and a Duration is a struct.
//
// They are rejected whether or not the breaker is enabled, as the tags do with the
// counters that are out of range.
func validateCircuitBreakerDurations(routes []Route) error {
	var err error
	for index := range routes {
		breaker := &routes[index].CircuitBreaker
		err = errors.Join(err,
			validateNotNegative(index, "Interval", breaker.Interval),
			validateNotNegative(index, "WaitDurationInOpenState", breaker.WaitDurationInOpenState))
	}
	return err
}

// validateNotNegative names the field as the validator does, by its path in the config.
func validateNotNegative(routeIndex int, field string, value Duration) error {
	if value.Duration >= 0 {
		return nil
	}
	return fmt.Errorf("%w: 'Config.Gateway.Routes[%d].CircuitBreaker.%s' is %s",
		ErrNegativeDuration, routeIndex, field, value.Duration)
}
