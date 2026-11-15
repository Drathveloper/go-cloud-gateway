package shared_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

var errOther = errors.New("other")

type derivedKey struct{}

func TestAsRouteTimeout(t *testing.T) {
	timedOut, cancelTimedOut := context.WithCancelCause(context.Background())
	cancelTimedOut(context.DeadlineExceeded)
	clientGone, cancelClientGone := context.WithCancel(context.Background())
	cancelClientGone()
	// A context derived from the one of a streaming route, as the one of a request is.
	derived := context.WithValue(timedOut, derivedKey{}, "value")
	wrappedCanceled := fmt.Errorf("stream error: %w", context.Canceled)

	tests := []struct {
		ctx          context.Context //nolint:containedctx
		err          error
		name         string
		wantDeadline bool
		wantCanceled bool
		wantSame     bool
	}{
		{name: "nil error", ctx: timedOut, err: nil, wantSame: true},
		{name: "canceled by the timeout", ctx: timedOut, err: context.Canceled, wantDeadline: true},
		{name: "wrapped canceled by the timeout", ctx: timedOut, err: wrappedCanceled, wantDeadline: true},
		{name: "canceled by the timeout of a parent context", ctx: derived, err: context.Canceled, wantDeadline: true},
		{name: "canceled because the client went away", ctx: clientGone, err: context.Canceled, wantCanceled: true, wantSame: true},
		{name: "canceled while the context is alive", ctx: context.Background(), err: context.Canceled, wantCanceled: true, wantSame: true},
		{name: "already a deadline", ctx: timedOut, err: context.DeadlineExceeded, wantDeadline: true, wantSame: true},
		{name: "unrelated error", ctx: timedOut, err: errOther, wantSame: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shared.AsRouteTimeout(tt.ctx, tt.err)

			if tt.wantSame && !errors.Is(got, tt.err) {
				t.Errorf("expected the error unchanged, got %v", got)
			}
			if tt.err == nil && got != nil {
				t.Errorf("expected nil, got %v", got)
			}
			if gotDeadline := errors.Is(got, context.DeadlineExceeded); gotDeadline != tt.wantDeadline {
				t.Errorf("errors.Is(DeadlineExceeded) = %v, want %v (%v)", gotDeadline, tt.wantDeadline, got)
			}
			if gotCanceled := errors.Is(got, context.Canceled); gotCanceled != tt.wantCanceled {
				t.Errorf("errors.Is(Canceled) = %v, want %v (%v)", gotCanceled, tt.wantCanceled, got)
			}
		})
	}
}

func TestAsRouteTimeout_KeepsTheMessage(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(context.DeadlineExceeded)

	got := shared.AsRouteTimeout(ctx, fmt.Errorf("stream error: %w", context.Canceled))

	if want := "context deadline exceeded: stream error: context canceled"; got.Error() != want {
		t.Errorf("got %q, want %q", got.Error(), want)
	}
}
