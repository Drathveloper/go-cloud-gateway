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

// readmeExample is a yaml block of the README.
type readmeExample struct {
	content string
	// line is the line of the README the block opens at. It tells the examples apart.
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

// readmeExamples returns the yaml blocks of the README. They are taken from the document
// itself rather than copied here, so that what the test builds is what a user copies.
func readmeExamples(t *testing.T) []readmeExample {
	t.Helper()
	examples := make([]readmeExample, 0)
	var block strings.Builder
	// openedAt is the line the current block opens at, zero outside a block.
	openedAt := 0
	for index, line := range strings.Split(readReadme(t), "\n") {
		fence := strings.TrimSpace(line)
		switch {
		case openedAt == 0 && fence == yamlFence:
			openedAt = index + 1
		case openedAt != 0 && fence == closingFence:
			examples = append(examples, readmeExample{content: block.String(), line: openedAt})
			block.Reset()
			openedAt = 0
		case openedAt != 0:
			block.WriteString(line + "\n")
		}
	}
	if openedAt != 0 {
		t.Fatalf("the yaml block that opens at line %d of %s is not closed", openedAt, readmePath)
	}
	return examples
}

func TestReadme_YAMLExamplesBuildAGateway(t *testing.T) {
	examples := readmeExamples(t)
	if len(examples) == 0 {
		t.Fatalf("expected yaml examples in %s actual none", readmePath)
	}
	reader := config.NewReaderYAML(validator.New())
	for _, example := range examples {
		t.Run(fmt.Sprintf("example at line %d", example.line), func(t *testing.T) {
			cfg, err := reader.Read([]byte(example.content))
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			// Reading only validates the shape of the configuration: the arguments of the
			// predicates and the filters are checked when the gateway is built.
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
