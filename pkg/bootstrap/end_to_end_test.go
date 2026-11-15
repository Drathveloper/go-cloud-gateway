package bootstrap_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

// The end-to-end tests run a gateway built by bootstrap.Initialize from a YAML
// configuration, on a real listener, in front of httptest backends. Their helpers are
// kept in this file, apart from the ones of bootstrap_test.go, so that the tests do not
// depend on that file.

// readYAMLConfig reads a YAML configuration with the reader a gateway is started with.
func readYAMLConfig(t *testing.T, cfgYAML string) *config.Config {
	t.Helper()
	cfg, err := config.NewReaderYAML(validator.New()).Read([]byte(cfgYAML))
	if err != nil {
		t.Fatalf("failed to read config: %v\n%s", err, cfgYAML)
	}
	return cfg
}

// newEndToEndGateway returns a test server, not started yet, that serves the server
// bootstrap.Initialize builds from the given options: it keeps its handler, its timeouts
// and its protocols. The caller starts it, with or without TLS.
func newEndToEndGateway(t *testing.T, opts *bootstrap.Options) *httptest.Server {
	t.Helper()
	server, err := bootstrap.Initialize(opts)
	if err != nil {
		t.Fatalf("failed to initialize gateway: %v", err)
	}
	testServer := httptest.NewUnstartedServer(server.Handler)
	testServer.Config = server
	t.Cleanup(testServer.Close)
	return testServer
}

// startEndToEndGateway starts a gateway built from the given YAML configuration and the
// default options.
func startEndToEndGateway(t *testing.T, cfgYAML string) *httptest.Server {
	t.Helper()
	gatewayServer := newEndToEndGateway(t, bootstrap.NewOptionsBuilder(readYAMLConfig(t, cfgYAML)).Build())
	gatewayServer.Start()
	return gatewayServer
}

// endToEndResponse is a response of the gateway, read to its end.
type endToEndResponse struct {
	header http.Header
	body   []byte
	status int
	// protoMajor is the major version of the protocol the gateway answered with.
	protoMajor int
	// contentLength is the length the response declared, -1 when it declared none.
	contentLength int64
}

// doEndToEndRequest sends the request and returns the response once its body is read.
func doEndToEndRequest(t *testing.T, client *http.Client, request *http.Request) endToEndResponse {
	t.Helper()
	res, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return endToEndResponse{
		header:        res.Header,
		body:          body,
		status:        res.StatusCode,
		protoMajor:    res.ProtoMajor,
		contentLength: res.ContentLength,
	}
}
