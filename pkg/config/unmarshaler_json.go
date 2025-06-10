package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// UnmarshalJSON implements the json.Unmarshaler interface for type Duration.
//
// The unmarshaler supports unmarshaling of float64 and string values.
//
// The unmarshaler supports unmarshaling of the following formats:
//
// 1. 30s
// 2. 30
// 3. 30.0
// 4. 30.000000000.
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
