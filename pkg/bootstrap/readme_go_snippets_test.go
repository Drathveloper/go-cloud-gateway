package bootstrap_test

import (
	"encoding/json"
	"fmt"
	"os"      //nolint:depguard // the snippets are written to files for the compiler to read
	"os/exec" //nolint:depguard // the snippets are compiled by the go command
	"strings"
	"testing"
)

const (
	// goFence opens the blocks of the README that hold Go code.
	goFence = "```go"
	// readmeSnippetMarker is the line of a wrapper the snippet is put in place of.
	readmeSnippetMarker = "// README SNIPPET"
	// packageClause starts a snippet that is a complete file.
	packageClause = "package "
	// compiledFileName is the name of the file a snippet is compiled as.
	compiledFileName = "snippet.go"
)

// readmeGoFragments tells how to compile the Go blocks of the README that are not a
// complete file. A block that starts with a package clause is compiled as it is. Any
// other block is a fragment, and has to be listed here, by its first line, with the file
// it is compiled in: the fragment takes the place of the readmeSnippetMarker line. The
// wrapper holds what the text around the fragment takes for granted, and nothing else:
// the package clause, the imports and the declarations the fragment refers to.
//
// There is no way to skip a block: a fragment without a wrapper fails the test, and so
// does a wrapper without a fragment.
var readmeGoFragments = map[string]string{
	// Bootstrap options: statements, with the configuration already read.
	"opts := bootstrap.NewOptionsBuilder(cfg).": `package snippet

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
)

func newServer(cfg *config.Config) (*http.Server, error) {
	// README SNIPPET
	return server, err
}
`,
	// Extending the gateway: declarations.
	"type stampFilter struct {": `package snippet

import (
	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// README SNIPPET
`,
	// Observing streamed bodies: a method of a filter the text does not declare.
	"func (f *SSEUsageLogger) PostProcess(ctx *gateway.Context) error {": `package snippet

import (
	"bytes"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type SSEUsageLogger struct{}

// README SNIPPET
`,
}

// lineDirective returns the directive that makes the compiler report the lines after it
// as the given line of the given file and the ones that follow, so that an error in a
// snippet is reported at its line of the README.
func lineDirective(file string, line int) string {
	return fmt.Sprintf("//line %s:%d:1\n", file, line)
}

// firstLine returns the first line of the snippet that is not blank.
func firstLine(snippet string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(snippet), "\n")
	return strings.TrimSpace(line)
}

// readmeSnippetSource returns the file the snippet is compiled as, and the key of the
// wrapper it was put in, empty for a snippet that is a complete file.
func readmeSnippetSource(snippet readmeBlock, readmeFile string) (string, string, error) {
	// The block opens at snippet.line, so its content starts on the next one.
	content := lineDirective(readmeFile, snippet.line+1) + snippet.content
	key := firstLine(snippet.content)
	if strings.HasPrefix(key, packageClause) {
		return content, "", nil
	}
	wrapper, found := readmeGoFragments[key]
	if !found {
		return "", "", fmt.Errorf("the go block at line %d of %s is not a complete file, as it has no package clause, "+
			"and readmeGoFragments has no wrapper for its first line %q: add one", snippet.line, readmePath, key)
	}
	lines := strings.SplitAfter(wrapper, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) != readmeSnippetMarker {
			continue
		}
		// The lines of the wrapper after the snippet are reported again as the lines of
		// the compiled file they are: the marker has given way to the first directive,
		// the snippet and this second directive.
		nextLine := index + 1 + strings.Count(content, "\n") + 1
		lines[index] = content + lineDirective(compiledFileName, nextLine)
		return strings.Join(lines, ""), key, nil
	}
	return "", "", fmt.Errorf("the wrapper of %q in readmeGoFragments has no %q line", key, readmeSnippetMarker)
}

// compileReadmeSnippet compiles the source as the only file of a package of this module,
// so that it resolves its imports as any other package of the module does, and returns
// the output of the compiler.
//
// The package only exists for the go command: the file is written to a temporary
// directory and an overlay places it in a directory under this one, which is never
// created. Nothing is written to the module.
func compileReadmeSnippet(t *testing.T, name, source string) ([]byte, error) {
	t.Helper()
	dir := t.TempDir()
	sourcePath, overlayPath := dir+"/"+compiledFileName, dir+"/overlay.json"
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {name + "/" + compiledFileName: sourcePath}})
	if err != nil {
		t.Fatalf("failed to encode overlay: %v", err)
	}
	for path, content := range map[string][]byte{sourcePath: []byte(source), overlayPath: overlay} {
		if err = os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}
	// go list -export compiles the package, which is what its export data comes from, and
	// stops there. go build would also link the snippet that is a program, which takes
	// longer than compiling all of them and proves nothing more about the snippet.
	//nolint:gosec // G204: a fixed command whose only variable arguments are a file of t.TempDir and the given name
	cmd := exec.CommandContext(t.Context(), "go", "list", "-export", "-overlay", overlayPath, "./"+name)
	return cmd.CombinedOutput() //nolint:wrapcheck
}

// The Go blocks of the README are compiled against this module, so that a snippet cannot
// keep using a name the library renamed or dropped.
func TestReadme_GoSnippetsCompile(t *testing.T) {
	t.Parallel()
	snippets := readmeBlocks(t, goFence)
	if len(snippets) == 0 {
		t.Fatalf("expected go snippets in %s actual none", readmePath)
	}
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get the working directory: %v", err)
	}
	// An absolute name, so that the compiler reports the errors at the README whatever
	// directory the snippet is compiled from.
	readmeFile := workDir + "/" + readmePath
	usedWrappers := make(map[string]bool)
	for _, snippet := range snippets {
		source, wrapper, err := readmeSnippetSource(snippet, readmeFile)
		if err != nil {
			t.Errorf("expected nil err actual %v", err)
			continue
		}
		if wrapper != "" {
			usedWrappers[wrapper] = true
		}
		t.Run(fmt.Sprintf("snippet at line %d", snippet.line), func(t *testing.T) {
			t.Parallel()

			output, err := compileReadmeSnippet(t, fmt.Sprintf("readme_snippet_at_line_%d", snippet.line), source)

			if err != nil {
				t.Errorf("expected the go block at line %d of %s to compile, actual %v:\n%s\ncompiled as:\n%s",
					snippet.line, readmePath, err, output, source)
			}
		})
	}
	for key := range readmeGoFragments {
		if !usedWrappers[key] {
			t.Errorf("expected a go block of %s that starts with %q, as readmeGoFragments has a wrapper for it, "+
				"actual none: remove the wrapper or update its key", readmePath, key)
		}
	}
}
