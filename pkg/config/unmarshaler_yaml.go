package config

import (
	"errors"
	"fmt"
	"time"
)

var ErrUnmarshalDuration = errors.New("unmarshal duration failed")

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
