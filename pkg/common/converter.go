package common

import (
	"errors"
	"fmt"
	"time"
)

var ErrRequiredValue = errors.New("value is required")
var ErrRequiredStringValue = errors.New("value is required to be a valid string")
var ErrRequiredSliceValue = errors.New("value is required to be a valid slice")

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
