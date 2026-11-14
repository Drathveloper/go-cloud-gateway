package config

import (
	"errors"
	"fmt"
	"time"
)

// ErrUnmarshalDuration is returned when the unmarshaler fails to unmarshal a duration.
var ErrUnmarshalDuration = errors.New("unmarshal duration failed")

// UnmarshalYAML implements the yaml.Unmarshaler interface for type Duration.
//
// Only a string in the time.ParseDuration format is accepted, such as 30s, 1m30s or
// 250ms. A bare number is rejected: see Duration.
func (d *Duration) UnmarshalYAML(unmarshal func(any2 any) error) error {
	var val any
	if err := unmarshal(&val); err != nil {
		return ErrUnmarshalDuration
	}
	switch value := val.(type) {
	case int, int64, uint64, float64:
		return newDurationWithoutUnitError(value)
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnmarshalDuration, err.Error())
		}
		return nil
	default:
		return fmt.Errorf("%w: invalid duration: %v", ErrUnmarshalDuration, val)
	}
}

// newDurationWithoutUnitError builds the error for a duration written as a bare number.
// It names the fix because the mistake is easy to make and the number alone is ambiguous.
func newDurationWithoutUnitError(value any) error {
	return fmt.Errorf(
		"%w: duration %v has no unit: use a string with a unit such as \"30s\"", ErrUnmarshalDuration, value)
}
