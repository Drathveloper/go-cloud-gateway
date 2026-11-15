package gateway_test

import (
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// closeCounter counts how many times it is closed.
type closeCounter struct {
	io.Reader

	closes atomic.Int64
}

func (c *closeCounter) Close() error {
	c.closes.Add(1)
	return nil
}

// An http transport closes the body of the request it was given from the goroutine that
// writes it, at any time and even after RoundTrip has returned, and the handler closes
// it as well: the two can run at once.
func TestReplayableBody_Close_Concurrent(t *testing.T) {
	source := &closeCounter{Reader: strings.NewReader("body")}
	body := gateway.NewReplayableBody(source, -1)
	var closers sync.WaitGroup
	start := make(chan struct{})
	for range 8 {
		closers.Go(func() {
			<-start
			_ = body.Close()
		})
	}

	close(start)
	closers.Wait()

	assert.Equal(t, int64(1), source.closes.Load())
}
