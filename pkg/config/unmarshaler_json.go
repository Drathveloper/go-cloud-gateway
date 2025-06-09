package config

import (
	"encoding/json"
	"fmt"
	"time"
)

func (d *Duration) UnmarshalJSON(b []byte) error {
	var val interface{}
	if err := json.Unmarshal(b, &val); err != nil {
		return fmt.Errorf("%w: %v", ErrUnmarshalDuration, err.Error())
	}

	switch value := val.(type) {
	case float64:
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
