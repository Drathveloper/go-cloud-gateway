package common

import (
	"fmt"
	"time"
)

func ConvertToString(val any) (string, error) {
	if val == nil {
		return "", fmt.Errorf("value is required")
	}
	valStr, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("value is required to be a valid string")
	}
	return valStr, nil
}

func ConvertToStringSlice(val any) ([]string, error) {
	if val == nil {
		return nil, fmt.Errorf("value is required")
	}
	valAnySlice, ok := val.([]any)
	if !ok {
		valStrSlice, okStrSlice := val.([]string)
		if !okStrSlice {
			return nil, fmt.Errorf("value is required to be a valid slice")
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
			return nil, fmt.Errorf("element at index %d is not of expected type", i)
		}
		result = append(result, val)
	}
	return result, nil
}

func ConvertToDateTime(val any) (time.Time, error) {
	if val == nil {
		return time.Time{}, fmt.Errorf("value is required")
	}
	valStr, ok := val.(string)
	if !ok {
		valTime, isOkTime := val.(time.Time)
		if !isOkTime {
			return time.Time{}, fmt.Errorf("value is required to be a valid string")
		}
		return valTime, nil
	}
	valDateTime, err := time.Parse(time.RFC3339, valStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("value is required to be a valid datetime: %w", err)
	}
	return valDateTime, nil
}
