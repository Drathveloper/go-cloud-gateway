package predicate

import (
	"fmt"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// AfterPredicateName is the name of the after predicate.
const AfterPredicateName = "After"

// BeforePredicateName is the name of the before predicate.
const BeforePredicateName = "Before"

// BetweenPredicateName is the name of the between predicate.
const BetweenPredicateName = "Between"

// AfterPredicate is a predicate that checks if the current time is after a given time.
type AfterPredicate struct {
	dateTime     time.Time
	timeProvider common.TimeProvider
}

// NewAfterPredicate creates a new after predicate.
//
// The time is always represented in UTC.
func NewAfterPredicate(dateTime time.Time) *AfterPredicate {
	return &AfterPredicate{
		dateTime:     dateTime.UTC(),
		timeProvider: &common.RealTime{},
	}
}

// NewAfterPredicateTest creates a new after predicate for tests.
func NewAfterPredicateTest(
	dateTime time.Time,
	provider common.TimeProvider) *AfterPredicate {
	return &AfterPredicate{
		dateTime:     dateTime.UTC(),
		timeProvider: provider,
	}
}

// NewAfterPredicateBuilder creates a new after predicate builder.
func NewAfterPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		dateTime, err := common.ConvertToDateTime(args["datetime"])
		if err != nil {
			return nil, fmt.Errorf("arg 'datetime' is required to be a valid datetime: %w", err)
		}
		return NewAfterPredicate(dateTime), nil
	}
}

// Test checks if the current time is after the given time.
//
// If the current time is after the given time, the predicate will return true.
// If the current time is before the given time, the predicate will return false.
//
// The time is always represented in UTC.
func (p *AfterPredicate) Test(_ *http.Request) bool {
	return p.timeProvider.Now().UTC().After(p.dateTime)
}

// BeforePredicate is a predicate that checks if the current time is before a given time.
type BeforePredicate struct {
	dateTime     time.Time
	timeProvider common.TimeProvider
}

// NewBeforePredicate creates a new before predicate.
//
// The time is always represented in UTC.
func NewBeforePredicate(dateTime time.Time) *BeforePredicate {
	return &BeforePredicate{
		dateTime:     dateTime.UTC(),
		timeProvider: &common.RealTime{},
	}
}

// NewBeforePredicateTest creates a new before predicate for tests.
func NewBeforePredicateTest(
	dateTime time.Time,
	provider common.TimeProvider) *BeforePredicate {
	return &BeforePredicate{
		dateTime:     dateTime.UTC(),
		timeProvider: provider,
	}
}

// NewBeforePredicateBuilder creates a new before predicate builder.
func NewBeforePredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		dateTime, err := common.ConvertToDateTime(args["datetime"])
		if err != nil {
			return nil, fmt.Errorf("arg 'datetime' is required to be a valid datetime: %w", err)
		}
		return NewBeforePredicate(dateTime), nil
	}
}

// Test checks if the current time is before the given time.
//
// If the current time is before the given time, the predicate will return true.
// If the current time is after the given time, the predicate will return false.
//
// The time is always represented in UTC.
func (p *BeforePredicate) Test(_ *http.Request) bool {
	return p.timeProvider.Now().UTC().Before(p.dateTime)
}

// BetweenPredicate is a predicate that checks if the current time is between a given time range.
type BetweenPredicate struct {
	startDateTime time.Time
	endDateTime   time.Time
	timeProvider  common.TimeProvider
}

// NewBetweenPredicate creates a new between predicate.
//
// The time is always represented in UTC.
func NewBetweenPredicate(startDateTime, endDateTime time.Time) *BetweenPredicate {
	return &BetweenPredicate{
		startDateTime: startDateTime.UTC(),
		endDateTime:   endDateTime.UTC(),
		timeProvider:  &common.RealTime{},
	}
}

// NewBetweenPredicateTest creates a new between predicate for tests.
func NewBetweenPredicateTest(
	startDateTime,
	endDateTime time.Time,
	provider common.TimeProvider) *BetweenPredicate {
	return &BetweenPredicate{
		startDateTime: startDateTime.UTC(),
		endDateTime:   endDateTime.UTC(),
		timeProvider:  provider,
	}
}

// NewBetweenPredicateBuilder creates a new between predicate builder.
func NewBetweenPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		startDateTime, err := common.ConvertToDateTime(args["start"])
		if err != nil {
			return nil, fmt.Errorf("arg 'start' is required to be a valid datetime: %w", err)
		}
		endDateTime, err := common.ConvertToDateTime(args["end"])
		if err != nil {
			return nil, fmt.Errorf("arg 'end' is required to be a valid datetime: %w", err)
		}
		return NewBetweenPredicate(startDateTime, endDateTime), nil
	}
}

// Test checks if the current time is between the given time.
//
// If the current time is between the given time, the predicate will return true.
// If the current time is not between the given time, the predicate will return false.
//
// The time is always represented in UTC.
func (p *BetweenPredicate) Test(_ *http.Request) bool {
	now := p.timeProvider.Now()
	return now.After(p.startDateTime) && now.Before(p.endDateTime)
}
