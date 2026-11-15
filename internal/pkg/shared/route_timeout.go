package shared

import (
	"context"
	"errors"
	"fmt"
)

// AsRouteTimeout returns err as a context.DeadlineExceeded when it is a context.Canceled
// that the timeout of the given context caused, and err unchanged otherwise.
//
// The context of a streaming route cannot be a deadline context, as its timeout has to
// be stopped once the response starts: the timeout cancels it with
// context.DeadlineExceeded as its cause, but its Err is context.Canceled. Code that
// reports ctx.Err() rather than context.Cause(ctx), as the HTTP/2 transport of net/http
// does, then makes the timeout look like a client that went away.
//
// The returned error does not match context.Canceled, which the circuit breaker does not
// count as a failure. A cancellation caused by anything else, such as the client going
// away, is left alone.
func AsRouteTimeout(ctx context.Context, err error) error {
	if err == nil ||
		!errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		!errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %s", context.DeadlineExceeded, err.Error())
}
