package predicate

import (
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

// optionalRegexpArg reads the 'regexp' argument shared by the header, query and cookie predicate builders.
//
// A missing regexp is not an error: it yields the empty regexp, which those predicates treat as
// an existence check.
func optionalRegexpArg(args map[string]any) (string, error) {
	value := args["regexp"]
	if value == nil {
		return "", nil
	}
	regex, err := shared.ConvertToString(value)
	if err != nil {
		return "", fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
	}
	return regex, nil
}
