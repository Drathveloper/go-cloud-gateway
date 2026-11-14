package config

import (
	"encoding/json"
	"fmt"
	"time"
)

// UnmarshalJSON implements the json.Unmarshaler interface for type Duration.
//
// Only a string in the time.ParseDuration format is accepted, such as "30s", "1m30s" or
// "250ms". A bare number is rejected: see Duration.
func (d *Duration) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return fmt.Errorf("%w: %v", ErrUnmarshalDuration, err.Error())
	}

	switch value := val.(type) {
	case float64:
		// the raw text is reported instead of the float so the user sees what was written
		return newDurationWithoutUnitError(string(data))
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
