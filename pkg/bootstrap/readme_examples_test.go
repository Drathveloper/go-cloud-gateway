package bootstrap_test

import (
	"fmt"
	"maps"
	"os" //nolint:depguard // the examples are read from README.md, a file outside any package
	"slices"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

const (
	// readmePath is relative to this package: go test runs in the package directory.
	readmePath = "../../README.md"
	// yamlFence opens the blocks of the README that hold a configuration example.
	yamlFence = "```yaml"
	// closingFence closes them.
	closingFence = "```"
)

// The placeholders of the README examples. An example cannot carry real PEM content, least
// of all a private key, so it has one of these lines where that content goes and the test
// puts there content it generates. A placeholder is a whole line in angle brackets.
const (
	clientCertificatePlaceholder = "<base64 of the client certificate>"
	clientKeyPlaceholder         = "<base64 of the private key of the client certificate>"
	authorityPlaceholder         = "<base64 of the certificate of the authority that signs the backend certificates>"
)

// readmeBlock is a fenced block of the README.
type readmeBlock struct {
	content string
	// line is the line of the README the block opens at. It tells the blocks apart.
	line int
}

func readReadme(t *testing.T) string {
	t.Helper()
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", readmePath, err)
	}
	return string(readme)
}

// readmeBlocks returns the blocks of the README that open with the given fence. They are
// taken from the document itself rather than copied here, so that what the test builds is
// what a user copies.
func readmeBlocks(t *testing.T, openingFence string) []readmeBlock {
	t.Helper()
	blocks := make([]readmeBlock, 0)
	var block strings.Builder
	// openedAt is the line the current block opens at, zero outside a block.
	openedAt := 0
	for index, line := range strings.Split(readReadme(t), "\n") {
		fence := strings.TrimSpace(line)
		switch {
		case openedAt == 0 && fence == openingFence:
			openedAt = index + 1
		case openedAt != 0 && fence == closingFence:
			blocks = append(blocks, readmeBlock{content: block.String(), line: openedAt})
			block.Reset()
			openedAt = 0
		case openedAt != 0:
			block.WriteString(line + "\n")
		}
	}
	if openedAt != 0 {
		t.Fatalf("the %s block that opens at line %d of %s is not closed", openingFence, openedAt, readmePath)
	}
	return blocks
}

// pemBody returns the base64 lines of a PEM block, without its BEGIN and END lines.
func pemBody(t *testing.T, pemText string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(pemText), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected a PEM block actual %q", pemText)
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}

// newReadmePlaceholderValues returns the content each placeholder is replaced with: a
// client certificate and its key, generated for the test, and the authority that signed it.
func newReadmePlaceholderValues(t *testing.T) map[string]string {
	t.Helper()
	authority := newTestAuthority(t, "authority of the readme examples")
	certificate := authority.issueClient(t, "client of the readme examples")
	return map[string]string{
		clientCertificatePlaceholder: pemBody(t, certificate.cert),
		clientKeyPlaceholder:         pemBody(t, certificate.key),
		authorityPlaceholder:         pemBody(t, authority.certPEM),
	}
}

// fillReadmePlaceholders returns the example with every placeholder line replaced by its
// value, indented like the placeholder. A placeholder without a value is an error: the
// example would otherwise be built with the placeholder itself as content.
func fillReadmePlaceholders(example string, values map[string]string) (string, error) {
	lines := strings.Split(example, "\n")
	for index, line := range lines {
		placeholder := strings.TrimSpace(line)
		if !strings.HasPrefix(placeholder, "<") || !strings.HasSuffix(placeholder, ">") {
			continue
		}
		value, known := values[placeholder]
		if !known {
			return "", fmt.Errorf("unknown placeholder %s at line %d of the example: the known ones are %q",
				placeholder, index+1, slices.Sorted(maps.Keys(values)))
		}
		indent, _, _ := strings.Cut(line, placeholder)
		lines[index] = indent + strings.ReplaceAll(value, "\n", "\n"+indent)
	}
	return strings.Join(lines, "\n"), nil
}

func TestReadme_YAMLExamplesBuildAGateway(t *testing.T) {
	examples := readmeBlocks(t, yamlFence)
	if len(examples) == 0 {
		t.Fatalf("expected yaml examples in %s actual none", readmePath)
	}
	reader := config.NewReaderYAML(validator.New())
	placeholderValues := newReadmePlaceholderValues(t)
	// A placeholder no example has is one the README dropped or renamed.
	for placeholder := range placeholderValues {
		hasPlaceholder := func(example readmeBlock) bool { return strings.Contains(example.content, placeholder) }
		if !slices.ContainsFunc(examples, hasPlaceholder) {
			t.Errorf("expected an example of %s with the placeholder %s actual none", readmePath, placeholder)
		}
	}
	for _, example := range examples {
		t.Run(fmt.Sprintf("example at line %d", example.line), func(t *testing.T) {
			content, err := fillReadmePlaceholders(example.content, placeholderValues)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			cfg, err := reader.Read([]byte(content))
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			// Reading only validates the shape of the configuration: the arguments of the
			// predicates and the filters are checked when the gateway is built, and so is
			// the PEM content of the mtls section.
			server, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())

			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			if server == nil {
				t.Fatalf("expected a server actual nil")
			}
		})
	}
}

func TestReadme_PlaceholdersAreFilled(t *testing.T) {
	values := map[string]string{"<first>": "one", "<second>": "two\nlines"}
	tests := []struct {
		name        string
		example     string
		expected    string
		expectedErr bool
	}{
		{
			name:     "placeholder line should be replaced by its value with the same indentation",
			example:  "key: |\n  BEGIN\n  <first>\n  END\n",
			expected: "key: |\n  BEGIN\n  one\n  END\n",
		},
		{
			name:     "every line of a value should be indented like the placeholder",
			example:  "a: |\n    <second>\nb: |\n  <first>\n",
			expected: "a: |\n    two\n    lines\nb: |\n  one\n",
		},
		{
			name:     "angle brackets within a line should not be a placeholder",
			example:  "regexp: ^/(?P<first>.*)$\nvalue: <unknown> text\n",
			expected: "regexp: ^/(?P<first>.*)$\nvalue: <unknown> text\n",
		},
		{
			name:        "placeholder without a value should be an error",
			example:     "key: |\n  <first>\n  <third>\n",
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filled, err := fillReadmePlaceholders(tt.example, values)

			if (err != nil) != tt.expectedErr {
				t.Fatalf("expected err %t actual %v", tt.expectedErr, err)
			}
			if filled != tt.expected {
				t.Errorf("expected %q actual %q", tt.expected, filled)
			}
		})
	}
}

func TestReadme_DocumentsEveryBuiltIn(t *testing.T) {
	tests := []struct {
		name  string
		names []string
	}{
		{
			name:  "readme should document every predicate",
			names: slices.Sorted(maps.Keys(predicate.BuilderRegistry)),
		},
		{
			name:  "readme should document every filter",
			names: slices.Sorted(maps.Keys(filter.BuilderRegistry)),
		},
		{
			name:  "readme should document every rate limiter",
			names: slices.Sorted(maps.Keys(ratelimit.RateLimiterBuilderRegistry)),
		},
		{
			name:  "readme should document every rate limit key",
			names: slices.Sorted(maps.Keys(ratelimit.KeyFuncBuilderRegistry)),
		},
	}
	readme := readReadme(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.names) == 0 {
				t.Fatalf("expected registered names actual none")
			}
			for _, name := range tt.names {
				// The reference tables write the names as inline code.
				if !strings.Contains(readme, "`"+name+"`") {
					t.Errorf("expected %s to mention `%s`", readmePath, name)
				}
			}
		})
	}
}
