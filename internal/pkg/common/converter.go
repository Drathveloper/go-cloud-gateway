package common

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrRequiredValue is returned when a value is required but is nil.
var ErrRequiredValue = errors.New("value is required")

// ErrRequiredStringValue is returned when a value is required to be a string but is not.
var ErrRequiredStringValue = errors.New("value is required to be a valid string")

// ErrRequiredIntValue is returned when a value is required to be an int but is not.
var ErrRequiredIntValue = errors.New("value is required to be a valid int")

// ErrRequiredSliceValue is returned when a value is required to be a slice but is not.
var ErrRequiredSliceValue = errors.New("value is required to be a valid slice")

// ConvertToString converts the given value to a string.
func ConvertToString(val any) (string, error) {
	if val == nil {
		return "", ErrRequiredValue
	}
	valStr, ok := val.(string)
	if !ok {
		return "", ErrRequiredStringValue
	}
	return valStr, nil
}

// ConvertToStringSlice converts the given value to a string slice.
func ConvertToStringSlice(val any) ([]string, error) {
	if val == nil {
		return nil, ErrRequiredValue
	}
	valAnySlice, ok := val.([]any)
	if !ok {
		valStrSlice, okStrSlice := val.([]string)
		if !okStrSlice {
			return nil, ErrRequiredSliceValue
		}
		return valStrSlice, nil
	}
	valStrSlice, err := ConvertSlice[string](valAnySlice)
	if err != nil {
		return nil, fmt.Errorf("value is expected to be a valid string slice: %w", err)
	}
	return valStrSlice, nil
}

// ConvertSlice converts the given value to a slice of the given type.
func ConvertSlice[T any](sliceAny []any) ([]T, error) {
	result := make([]T, 0, len(sliceAny))
	for i, item := range sliceAny {
		val, ok := item.(T)
		if !ok {
			return nil, fmt.Errorf("%w: element at index %d is not of expected type", ErrRequiredSliceValue, i)
		}
		result = append(result, val)
	}
	return result, nil
}

// ConvertToDateTime converts the given value to a time.Time.
//
// The value can be a string or a time.Time.
//
// The string value is expected to be a valid RFC3339 datetime.
//
// The time.Time value is returned as is.
func ConvertToDateTime(val any) (time.Time, error) {
	if val == nil {
		return time.Time{}, ErrRequiredValue
	}
	valStr, ok := val.(string)
	if !ok {
		valTime, isOkTime := val.(time.Time)
		if !isOkTime {
			return time.Time{}, ErrRequiredStringValue
		}
		return valTime, nil
	}
	valDateTime, err := time.Parse(time.RFC3339, valStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("value is required to be a valid datetime: %w", err)
	}
	return valDateTime, nil
}

// ConvertToInt converts the given value to an int.
//
// The value can be a string or an int.
//
// The string value is expected to be a valid int.
//
// The int value is returned as is.
//
// The function returns an error if the value is not a string or int.
func ConvertToInt(val any) (int, error) {
	if val == nil {
		return 0, ErrRequiredValue
	}
	if valInt, ok := val.(int); ok {
		return valInt, nil
	}
	valStr, ok := val.(string)
	if !ok {
		return 0, ErrRequiredIntValue
	}
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, ErrRequiredIntValue
	}
	return valInt, nil
}
