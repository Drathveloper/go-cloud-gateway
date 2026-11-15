package bootstrap_test

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

const (
	// backendHeader is the response header a test backend names itself in.
	backendHeader = "X-Backend"
	// backendTargetHeader is the response header a test backend echoes the request target
	// it received in, exactly as it came on the wire.
	backendTargetHeader = "X-Backend-Request-Target"
)

// predicateQuirksConfig routes to a backend per route, named like the route. The uri of
// each route is filled in with the url of its backend.
const predicateQuirksConfig = `
gateway:
  routes:
    - id: tenant
      uri: <tenant>
      predicates:
        - name: Header
          args:
            name: host
            regexp: ^tenant-[a-z]+\.example\.org(:[0-9]+)?$
    - id: chunked
      uri: <chunked>
      predicates:
        - name: Path
          args:
            patterns:
              - /upload
        - name: Header
          args:
            name: Transfer-Encoding
            regexp: ^chunked$
    - id: versioned
      uri: <versioned>
      predicates:
        - name: Path
          args:
            patterns:
              - /versioned
        - name: Header
          args:
            name: X-Api-Version
            regexp: ^v[0-9]+$
    - id: stylesheet
      uri: <stylesheet>
      predicates:
        - name: Path
          args:
            patterns:
              - /static/*.css
    - id: nested-stylesheet
      uri: <nested-stylesheet>
      predicates:
        - name: Path
          args:
            patterns:
              - /static/*/b.css
    - id: files
      uri: <files>
      predicates:
        - name: Path
          args:
            patterns:
              - /files/my file
    - id: public
      uri: <public>
      predicates:
        - name: Path
          args:
            patterns:
              - /public/*
    - id: admin
      uri: <admin>
      predicates:
        - name: Path
          args:
            patterns:
              - /admin/**
    - id: rewritten
      uri: <rewritten>
      predicates:
        - name: Path
          args:
            patterns:
              - /rewritten/*
      filters:
        - name: RewritePath
          args:
            regexp: ^/rewritten/(.*)$
            replacement: /$1
`

// receivedRequest is what the server of the gateway handed to its handler for a request.
type receivedRequest struct {
	header           http.Header
	target           string
	host             string
	proto            string
	transferEncoding []string
}

// requestRecorder keeps the last request the gateway received, as net/http parsed it and
// before the gateway touches it. It is how a test knows what its client really put on the
// wire, and what net/http made of it.
type requestRecorder struct {
	next http.Handler
	last receivedRequest
	mu   sync.Mutex
}

func (r *requestRecorder) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	r.mu.Lock()
	r.last = receivedRequest{
		header:           request.Header.Clone(),
		target:           request.RequestURI,
		host:             request.Host,
		proto:            request.Proto,
		transferEncoding: slices.Clone(request.TransferEncoding),
	}
	r.mu.Unlock()
	r.next.ServeHTTP(writer, request)
}

func (r *requestRecorder) received() receivedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.last
}

// newNamedBackend returns a backend that answers with its name and with the request
// target it received.
func newNamedBackend(t *testing.T, name string) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.Header().Set(backendHeader, name)
		writer.Header().Set(backendTargetHeader, request.RequestURI)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)
	return backend
}

// readPredicateQuirksConfig reads predicateQuirksConfig with the yaml reader, after
// starting the backend of every route.
func readPredicateQuirksConfig(t *testing.T) *config.Config {
	t.Helper()
	cfgYAML := predicateQuirksConfig
	for _, name := range []string{
		"tenant", "chunked", "versioned", "stylesheet", "nested-stylesheet", "files", "public", "admin", "rewritten",
	} {
		cfgYAML = strings.ReplaceAll(cfgYAML, "<"+name+">", newNamedBackend(t, name).URL)
	}
	cfg, err := config.NewReaderYAML(validator.New()).Read([]byte(cfgYAML))
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	return cfg
}

// startRecordedGateway serves the gateway built from the given config, over HTTP/1.1 or,
// with TLS, over HTTP/2. The handler of the server is the one bootstrap.Initialize
// returns, mux included, behind a recorder.
func startRecordedGateway(t *testing.T, cfg *config.Config, useHTTP2 bool) (*httptest.Server, *requestRecorder) {
	t.Helper()
	server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())
	if err != nil {
		t.Fatalf("failed to initialize gateway: %v", err)
	}
	recorder := &requestRecorder{next: server.Handler}
	server.Handler = recorder
	testServer := httptest.NewUnstartedServer(recorder)
	testServer.Config = server
	if useHTTP2 {
		testServer.EnableHTTP2 = true
		testServer.StartTLS()
	} else {
		testServer.Start()
	}
	t.Cleanup(testServer.Close)
	return testServer, recorder
}

// unsizedBody hides the length of a body from the client, which then has to send it
// chunked over HTTP/1.1.
type unsizedBody struct {
	io.Reader
}

type predicateQuirkCase struct {
	header http.Header
	// body is sent as it is when sized is true, and without a known length otherwise.
	body string
	name string
	// target is the request target, as it has to go on the wire.
	target string
	host   string
	method string
	// expectedBackend is the backend that must answer, empty when no route must match.
	expectedBackend string
	// expectedBackendTarget is the request target the backend must receive, when it is
	// not the one the client sent.
	expectedBackendTarget string
	// expectedLocation is where the mux must redirect to, empty when it must not.
	expectedLocation string
	// expectedTransferEncoding is what net/http must report in Request.TransferEncoding.
	expectedTransferEncoding []string
	// expectedHeaderLines is the number of values of X-Api-Version the gateway must
	// receive: one per header line.
	expectedHeaderLines int
	sized               bool
	// onlyHTTP1 marks a request that HTTP/2 cannot express.
	onlyHTTP1 bool
}

func predicateQuirkCases() []predicateQuirkCase {
	return []predicateQuirkCase{
		{
			name:            "header predicate on host should match the host of the request",
			method:          http.MethodGet,
			target:          "/anything",
			host:            "tenant-a.example.org",
			expectedBackend: "tenant",
		},
		{
			name:            "header predicate on host should match the host with the port the client sent",
			method:          http.MethodGet,
			target:          "/anything",
			host:            "tenant-a.example.org:8443",
			expectedBackend: "tenant",
		},
		{
			name:            "header predicate on host shouldn't match another host",
			method:          http.MethodGet,
			target:          "/anything",
			host:            "www.example.org",
			expectedBackend: "",
		},
		{
			name:                     "header predicate on transfer encoding should match a chunked request",
			method:                   http.MethodPost,
			target:                   "/upload",
			body:                     "payload",
			sized:                    false,
			expectedBackend:          "chunked",
			expectedTransferEncoding: []string{"chunked"},
			onlyHTTP1:                true,
		},
		{
			name:            "header predicate on transfer encoding shouldn't match a request with content length",
			method:          http.MethodPost,
			target:          "/upload",
			body:            "payload",
			sized:           true,
			expectedBackend: "",
		},
		{
			name:                "header predicate should match a value sent in a line of its own",
			method:              http.MethodGet,
			target:              "/versioned",
			header:              http.Header{"X-Api-Version": {"beta", "v3"}},
			expectedBackend:     "versioned",
			expectedHeaderLines: 2,
		},
		{
			name:                "header predicate should match the same value sent in a comma-separated line",
			method:              http.MethodGet,
			target:              "/versioned",
			header:              http.Header{"X-Api-Version": {"beta, v3"}},
			expectedBackend:     "versioned",
			expectedHeaderLines: 1,
		},
		{
			name:                "header predicate shouldn't match when no element of the line matches",
			method:              http.MethodGet,
			target:              "/versioned",
			header:              http.Header{"X-Api-Version": {"beta, v3x"}},
			expectedBackend:     "",
			expectedHeaderLines: 1,
		},
		{
			name:                "header predicate shouldn't split a quoted string",
			method:              http.MethodGet,
			target:              "/versioned",
			header:              http.Header{"X-Api-Version": {`"beta, v3"`}},
			expectedBackend:     "",
			expectedHeaderLines: 1,
		},
		{
			name:            "path predicate should keep an encoded slash inside its segment",
			method:          http.MethodGet,
			target:          "/static/a%2Fb.css",
			expectedBackend: "stylesheet",
		},
		{
			name:            "path predicate should split the segments at a real slash",
			method:          http.MethodGet,
			target:          "/static/a/b.css",
			expectedBackend: "nested-stylesheet",
		},
		{
			name:            "path predicate should match a literal pattern when the client encodes a character of it",
			method:          http.MethodGet,
			target:          "/files/my%20file",
			expectedBackend: "files",
		},
		{
			// The mux redirects a real dot segment, but an encoded slash hides this one
			// from it. The gateway does not resolve it either: it stays inside the
			// segment that /public/* matches, and the backend receives it as it came.
			name:            "path predicate should keep a dot segment with an encoded slash inside its segment",
			method:          http.MethodGet,
			target:          "/public/..%2Fadmin",
			expectedBackend: "public",
		},
		{
			// An encoded dot segment is not redirected by the mux either. It is a segment
			// like any other: three of them do not match /public/*, and the first one
			// keeps the request away from /admin/**.
			name:            "path predicate shouldn't resolve an encoded dot segment",
			method:          http.MethodGet,
			target:          "/public/%2E%2E/admin",
			expectedBackend: "",
		},
		{
			name:             "mux should redirect a dot segment before any route is tried",
			method:           http.MethodGet,
			target:           "/public/../admin/users",
			expectedLocation: "/admin/users",
		},
		{
			name:            "path predicate should match the path the mux redirects a dot segment to",
			method:          http.MethodGet,
			target:          "/admin/users",
			expectedBackend: "admin",
		},
		{
			// RewritePath works on the decoded path and builds a new one out of it: the
			// route is chosen with the encoded slash inside its segment, but the path the
			// filter writes has a real one.
			name:                  "rewrite path should send a real slash where the client sent an encoded one",
			method:                http.MethodGet,
			target:                "/rewritten/a%2Fb",
			expectedBackend:       "rewritten",
			expectedBackendTarget: "/a/b",
		},
	}
}

func (c *predicateQuirkCase) newRequest(t *testing.T, gatewayURL string) *http.Request {
	t.Helper()
	var body io.Reader
	switch {
	case c.body != "" && c.sized:
		body = strings.NewReader(c.body)
	case c.body != "":
		body = unsizedBody{Reader: strings.NewReader(c.body)}
	}
	request, err := http.NewRequestWithContext(t.Context(), c.method, gatewayURL+c.target, body)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if c.host != "" {
		request.Host = c.host
	}
	// Every value is a header line of its own: the client does not fold them.
	maps.Copy(request.Header, c.header)
	return request
}

// assertReceived checks that the request reached the gateway the way the test means it:
// the client could have normalized the request target or folded the header lines.
func (c *predicateQuirkCase) assertReceived(t *testing.T, received receivedRequest, proto string) {
	t.Helper()
	if received.proto != proto {
		t.Errorf("expected gateway to receive protocol %s actual %s", proto, received.proto)
	}
	if received.target != c.target {
		t.Errorf("expected gateway to receive request target %s actual %s", c.target, received.target)
	}
	if c.host != "" && received.host != c.host {
		t.Errorf("expected gateway to receive host %s actual %s", c.host, received.host)
	}
	// What net/http moves out of the headers is not in them, on any protocol.
	for _, name := range []string{"Host", "Transfer-Encoding"} {
		if values, ok := received.header[name]; ok {
			t.Errorf("expected gateway to receive no %s header actual %v", name, values)
		}
	}
	if !slices.Equal(received.transferEncoding, c.expectedTransferEncoding) {
		t.Errorf("expected gateway to receive transfer encoding %v actual %v",
			c.expectedTransferEncoding, received.transferEncoding)
	}
	if lines := len(received.header["X-Api-Version"]); lines != c.expectedHeaderLines {
		t.Errorf("expected gateway to receive %d X-Api-Version lines actual %d", c.expectedHeaderLines, lines)
	}
}

func (c *predicateQuirkCase) assertResponse(t *testing.T, response *http.Response) {
	t.Helper()
	expectedStatus, expectedBackendTarget := http.StatusOK, c.target
	switch {
	case c.expectedLocation != "":
		expectedStatus, expectedBackendTarget = http.StatusTemporaryRedirect, ""
	case c.expectedBackend == "":
		expectedStatus, expectedBackendTarget = http.StatusNotFound, ""
	case c.expectedBackendTarget != "":
		expectedBackendTarget = c.expectedBackendTarget
	}
	if response.StatusCode != expectedStatus {
		t.Errorf("expected status %d actual %d", expectedStatus, response.StatusCode)
	}
	if location := response.Header.Get("Location"); location != c.expectedLocation {
		t.Errorf("expected location %q actual %q", c.expectedLocation, location)
	}
	if backend := response.Header.Get(backendHeader); backend != c.expectedBackend {
		t.Errorf("expected backend %q actual %q", c.expectedBackend, backend)
	}
	if backendTarget := response.Header.Get(backendTargetHeader); backendTarget != expectedBackendTarget {
		t.Errorf("expected backend to receive request target %q actual %q", expectedBackendTarget, backendTarget)
	}
}

// TestInitialize_PredicateQuirks drives a whole gateway over the wire: read from yaml,
// built by bootstrap.Initialize and served behind its mux, in front of real backends.
func TestInitialize_PredicateQuirks(t *testing.T) {
	protocols := []struct {
		proto    string
		useHTTP2 bool
	}{
		{proto: "HTTP/1.1", useHTTP2: false},
		{proto: "HTTP/2.0", useHTTP2: true},
	}
	for _, protocol := range protocols {
		gatewayServer, recorder := startRecordedGateway(t, readPredicateQuirksConfig(t), protocol.useHTTP2)
		client := gatewayServer.Client()
		// A redirect of the mux is part of what is tested: it must not be followed.
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		for _, tt := range predicateQuirkCases() {
			if tt.onlyHTTP1 && protocol.useHTTP2 {
				continue
			}
			t.Run(protocol.proto+" "+tt.name, func(t *testing.T) {
				response, err := client.Do(tt.newRequest(t, gatewayServer.URL))
				if err != nil {
					t.Fatalf("request failed: %v", err)
				}
				defer response.Body.Close() //nolint:errcheck
				_, _ = io.Copy(io.Discard, response.Body)

				tt.assertReceived(t, recorder.received(), protocol.proto)
				tt.assertResponse(t, response)
			})
		}
	}
}

func TestInitialize_PredicateQuirks_RejectedHeaderNames(t *testing.T) {
	tests := []struct {
		name        string
		headerName  string
		expectedErr string
	}{
		{
			name:       "initialize should fail when a header predicate is on trailer",
			headerName: "trailer",
			expectedErr: `unavailable header "trailer": net/http moves it out of the request headers ` +
				"and does not keep the value the client sent",
		},
		{
			name:        "initialize should fail when a header predicate is on a pseudo-header",
			headerName:  `":authority"`,
			expectedErr: `invalid header name ":authority": it is not a header field name, so no request can carry it`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgYAML := `
gateway:
  routes:
    - id: uploads
      uri: http://localhost:8080
      predicates:
        - name: Header
          args:
            name: ` + tt.headerName + `
`
			cfg, err := config.NewReaderYAML(validator.New()).Read([]byte(cfgYAML))
			if err != nil {
				t.Fatalf("failed to read config: %v", err)
			}

			server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())

			if err == nil || !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("expected err with %q actual %v", tt.expectedErr, err)
			}
			if server != nil {
				t.Errorf("expected nil server actual %v", server)
			}
		})
	}
}
