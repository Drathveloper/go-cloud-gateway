package gateway_test

import (
	"bytes"
	"io"
	"strconv"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

var streamSizes = []int{
	1024,        // 1KB
	32 * 1024,   // 32KB, one copy-buffer read
	1024 * 1024, // 1MB
}

// plainBody is an io.ReadCloser over a byte slice that deliberately does NOT implement
// io.WriterTo, mirroring a real backend response body. A bare bytes.Reader exposes a
// WriteTo fast path that skips the per-chunk copy loop the unobserved gateway path
// actually runs, which would make the baseline incomparable to the observed path.
type plainBody struct {
	reader *bytes.Reader
}

func newPlainBody(data []byte) *plainBody {
	return &plainBody{reader: bytes.NewReader(data)}
}

func (p *plainBody) Read(output []byte) (int, error) {
	return p.reader.Read(output) //nolint:wrapcheck
}

func (p *plainBody) Close() error {
	return nil
}

// BenchmarkReplayableBody_WriteTo_Baseline measures the unobserved streaming path. It is
// the comparison anchor: ObserveStream must not change this number when no observer is
// registered, since it never touches rb.original in that case.
func BenchmarkReplayableBody_WriteTo_Baseline(b *testing.B) {
	for _, size := range streamSizes {
		body := bytes.Repeat([]byte("a"), size)
		b.Run(strconv.Itoa(size)+"_bytes", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				rb := gateway.NewReplayableBody(newPlainBody(body), int64(size))
				if _, err := rb.WriteTo(io.Discard); err != nil {
					b.Fatalf("writeTo failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkReplayableBody_WriteTo_Observed measures the per-chunk observation overhead
// (one mutex pair and the callback call per copy-buffer read) with no-op callbacks.
func BenchmarkReplayableBody_WriteTo_Observed(b *testing.B) {
	observerCounts := []int{1, 2}
	for _, observers := range observerCounts {
		for _, size := range streamSizes {
			body := bytes.Repeat([]byte("a"), size)
			name := strconv.Itoa(observers) + "_observers/" + strconv.Itoa(size) + "_bytes"
			b.Run(name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rb := gateway.NewReplayableBody(newPlainBody(body), int64(size))
					for range observers {
						rb.ObserveStream(func([]byte) {}, func(int64, error) {})
					}
					if _, err := rb.WriteTo(io.Discard); err != nil {
						b.Fatalf("writeTo failed: %v", err)
					}
				}
			})
		}
	}
}

// BenchmarkObserveStream_Register isolates the registration cost: one wrapper allocation
// per observing filter.
func BenchmarkObserveStream_Register(b *testing.B) {
	body := bytes.Repeat([]byte("a"), 1024)
	b.ReportAllocs()
	for b.Loop() {
		rb := gateway.NewReplayableBody(newPlainBody(body), int64(len(body)))
		rb.ObserveStream(func([]byte) {}, func(int64, error) {})
	}
}
