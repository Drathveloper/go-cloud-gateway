package bootstrap_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// clientCertificateHeader is the response header the mTLS backend tells the common name
// of the client certificate it was presented with.
const clientCertificateHeader = "X-Client-Certificate"

// pemCertificate is a certificate and its private key, PEM encoded as the mtls section of
// the configuration takes them.
type pemCertificate struct {
	cert string
	key  string
}

// testAuthority is a certificate authority made for a test. Nothing it issues outlives
// the test: the keys are generated on every run and are never written anywhere.
type testAuthority struct {
	certificate *x509.Certificate
	key         *ecdsa.PrivateKey
	// certPEM is the certificate of the authority, PEM encoded.
	certPEM string
}

func newTestKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

// newTestCertificateTemplate returns the fields every certificate of the tests shares.
// The serial number is left out: x509.CreateCertificate generates one.
func newTestCertificateTemplate(commonName string) *x509.Certificate {
	return &x509.Certificate{
		Subject:   pkix.Name{CommonName: commonName},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
	}
}

func encodeCertificatePEM(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func newTestAuthority(t *testing.T, commonName string) *testAuthority {
	t.Helper()
	key := newTestKey(t)
	template := newTestCertificateTemplate(commonName)
	template.IsCA = true
	template.BasicConstraintsValid = true
	template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create authority certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse authority certificate: %v", err)
	}
	return &testAuthority{certificate: certificate, key: key, certPEM: encodeCertificatePEM(der)}
}

// issue returns a certificate signed by the authority for the given template.
func (a *testAuthority) issue(t *testing.T, template *x509.Certificate) pemCertificate {
	t.Helper()
	key := newTestKey(t)
	der, err := x509.CreateCertificate(rand.Reader, template, a.certificate, &key.PublicKey, a.key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to encode key: %v", err)
	}
	return pemCertificate{
		cert: encodeCertificatePEM(der),
		key:  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
	}
}

// issueClient returns a certificate a client can present.
func (a *testAuthority) issueClient(t *testing.T, commonName string) pemCertificate {
	t.Helper()
	template := newTestCertificateTemplate(commonName)
	template.KeyUsage = x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	return a.issue(t, template)
}

// issueLoopbackServer returns a certificate for a server that listens on the loopback
// interface, as the httptest ones do.
func (a *testAuthority) issueLoopbackServer(t *testing.T) pemCertificate {
	t.Helper()
	template := newTestCertificateTemplate("backend")
	template.KeyUsage = x509.KeyUsageDigitalSignature
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	template.DNSNames = []string{"localhost"}
	template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	return a.issue(t, template)
}

// newMTLSBackend returns a TLS backend that presents the given certificate and only
// accepts the clients that present one signed by the given authority. It answers with the
// common name of the client certificate, and sends it to the returned channel as well.
func newMTLSBackend(
	t *testing.T,
	certificate pemCertificate,
	clientAuthority *testAuthority,
) (*httptest.Server, <-chan string) {
	t.Helper()
	keyPair, err := tls.X509KeyPair([]byte(certificate.cert), []byte(certificate.key))
	if err != nil {
		t.Fatalf("failed to load backend certificate: %v", err)
	}
	clientAuthorities := x509.NewCertPool()
	clientAuthorities.AddCert(clientAuthority.certificate)
	clients := make(chan string, 1)
	backend := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		commonName := request.TLS.PeerCertificates[0].Subject.CommonName
		clients <- commonName
		writer.Header().Set(clientCertificateHeader, commonName)
	}))
	backend.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{keyPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientAuthorities,
	}
	// The handshakes the backend rejects are the point of the test, not noise for its output.
	backend.Config.ErrorLog = log.New(io.Discard, "", 0)
	backend.StartTLS()
	t.Cleanup(backend.Close)
	return backend, clients
}

// yamlBlock returns the text as the content of a YAML block scalar indented with the
// given prefix.
func yamlBlock(text, indent string) string {
	return indent + strings.ReplaceAll(strings.TrimSuffix(text, "\n"), "\n", "\n"+indent)
}

// mtlsYAML returns an enabled mtls section that presents the given certificate and
// verifies the backends against the given authority.
func mtlsYAML(certificate pemCertificate, backendAuthority *testAuthority) string {
	const indent = "        "
	return fmt.Sprintf(`
    mtls:
      enabled: true
      cert: |
%s
      key: |
%s
      ca: |
%s`, yamlBlock(certificate.cert, indent), yamlBlock(certificate.key, indent), yamlBlock(backendAuthority.certPEM, indent))
}

// With mtls enabled the gateway presents its client certificate to the backends and
// verifies theirs against the configured authority. A backend that requires a client
// certificate cannot be called otherwise: the handshake fails and the gateway answers
// 502 Bad Gateway.
func TestEndToEnd_MTLS(t *testing.T) {
	const gatewayName = "go-cloud-gateway"
	authority := newTestAuthority(t, "authority of the test")
	otherAuthority := newTestAuthority(t, "another authority")
	backendCertificate := authority.issueLoopbackServer(t)
	tests := []struct {
		name       string
		httpClient string
		// expectedClient is the common name of the client certificate the backend must
		// be presented with, empty when the request must not reach the backend.
		expectedClient string
		expectedStatus int
	}{
		{
			name:           "gateway with a client certificate the backend trusts should reach the backend",
			httpClient:     mtlsYAML(authority.issueClient(t, gatewayName), authority),
			expectedClient: gatewayName,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "gateway without mtls section should be rejected by the backend",
			httpClient:     "\n    insecure-tls-verify: true",
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "gateway with mtls disabled should be rejected by the backend",
			httpClient:     "\n    insecure-tls-verify: true\n    mtls:\n      enabled: false",
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "gateway with a client certificate the backend does not trust should be rejected by the backend",
			httpClient:     mtlsYAML(otherAuthority.issueClient(t, gatewayName), authority),
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "gateway that does not trust the authority of the backend should not call it",
			httpClient:     mtlsYAML(authority.issueClient(t, gatewayName), otherAuthority),
			expectedStatus: http.StatusBadGateway,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, clients := newMTLSBackend(t, backendCertificate, authority)
			gatewayServer := startEndToEndGateway(t, fmt.Sprintf(`
gateway:
  httpclient:%s
  routes:
    - id: mtls
      uri: %s
`, tt.httpClient, backend.URL))
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gatewayServer.URL+"/test", nil)
			if err != nil {
				t.Fatalf("failed to build request: %v", err)
			}

			res := doEndToEndRequest(t, gatewayServer.Client(), request)

			if res.status != tt.expectedStatus {
				t.Errorf("expected status %d actual %d", tt.expectedStatus, res.status)
			}
			if client := res.header.Get(clientCertificateHeader); client != tt.expectedClient {
				t.Errorf("expected the backend to answer to the client certificate %q actual %q", tt.expectedClient, client)
			}
			// The backend closes before the channel is read, so a request that got to
			// its handler has already been recorded.
			backend.Close()
			select {
			case client := <-clients:
				if client != tt.expectedClient {
					t.Errorf("expected the backend to be called with the client certificate %q actual %q",
						tt.expectedClient, client)
				}
			default:
				if tt.expectedClient != "" {
					t.Errorf("expected the backend to be called actual not called")
				}
			}
		})
	}
}
