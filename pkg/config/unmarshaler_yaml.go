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
// The unmarshaler supports unmarshaling of float64 and string values.
//
// The unmarshaler supports unmarshaling of the following formats:
//
// 1. 30s
// 2. 30
// 3. 30.0
// 4. 30.000000000.
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val interface{}
	if err := unmarshal(&val); err != nil {
		return ErrUnmarshalDuration
	}
	switch value := val.(type) {
	case int:
		d.Duration = time.Duration(value)
		return nil
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
