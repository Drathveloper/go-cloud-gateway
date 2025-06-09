package predicate

import (
	"fmt"
	"net/http"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const AfterPredicateName = "After"
const BeforePredicateName = "Before"
const BetweenPredicateName = "Between"

type AfterPredicate struct {
	DateTime     time.Time
	timeProvider common.TimeProvider
}

func NewAfterPredicate(dateTime time.Time) *AfterPredicate {
	return &AfterPredicate{
		DateTime:     dateTime.UTC(),
		timeProvider: &common.RealTime{},
	}
}

func NewAfterPredicateTest(
	dateTime time.Time,
	provider common.TimeProvider) *AfterPredicate {
	return &AfterPredicate{
		DateTime:     dateTime.UTC(),
		timeProvider: provider,
	}
}

func NewAfterPredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		dateTime, err := common.ConvertToDateTime(args["datetime"])
		if err != nil {
			return nil, fmt.Errorf("arg 'datetime' is required to be a valid datetime: %w", err)
		}
		return NewAfterPredicate(dateTime), nil
	}
}

func (p *AfterPredicate) Test(_ *http.Request) bool {
	return p.timeProvider.Now().UTC().After(p.DateTime)
}

type BeforePredicate struct {
	DateTime     time.Time
	timeProvider common.TimeProvider
}

func NewBeforePredicate(dateTime time.Time) *BeforePredicate {
	return &BeforePredicate{
		DateTime:     dateTime.UTC(),
		timeProvider: &common.RealTime{},
	}
}

func NewBeforePredicateTest(
	dateTime time.Time,
	provider common.TimeProvider) *BeforePredicate {
	return &BeforePredicate{
		DateTime:     dateTime.UTC(),
		timeProvider: provider,
	}
}

func NewBeforePredicateBuilder() gateway.PredicateBuilderFunc {
	return func(args map[string]any) (gateway.Predicate, error) {
		dateTime, err := common.ConvertToDateTime(args["datetime"])
		if err != nil {
			return nil, fmt.Errorf("arg 'datetime' is required to be a valid datetime: %w", err)
		}
		return NewBeforePredicate(dateTime), nil
	}
}

func (p *BeforePredicate) Test(_ *http.Request) bool {
	return p.timeProvider.Now().UTC().Before(p.DateTime)
}

type BetweenPredicate struct {
	StartDateTime time.Time
	EndDateTime   time.Time
	timeProvider  common.TimeProvider
}

func NewBetweenPredicate(startDateTime, endDateTime time.Time) *BetweenPredicate {
	return &BetweenPredicate{
		StartDateTime: startDateTime.UTC(),
		EndDateTime:   endDateTime.UTC(),
		timeProvider:  &common.RealTime{},
	}
}

func NewBetweenPredicateTest(
	startDateTime,
	endDateTime time.Time,
	provider common.TimeProvider) *BetweenPredicate {
	return &BetweenPredicate{
		StartDateTime: startDateTime.UTC(),
		EndDateTime:   endDateTime.UTC(),
		timeProvider:  provider,
	}
}

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

func (p *BetweenPredicate) Test(_ *http.Request) bool {
	now := p.timeProvider.Now()
	return now.After(p.StartDateTime) && now.Before(p.EndDateTime)
}
