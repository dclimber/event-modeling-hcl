package interchange

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseDocument decodes one event model that conforms to the published
// interchange schema. Any schema violation rejects the whole document, so the
// importer never has to guess at missing, unknown or out-of-enum content.
func ParseDocument(data []byte) (*Document, error) {
	_, violations := conform(data)
	if len(violations) > 0 {
		return nil, fmt.Errorf("event model JSON does not conform to the interchange schema:\n  %s", strings.Join(violations, "\n  "))
	}
	// Decode the original bytes, not a re-marshalled copy: re-encoding a
	// generic value would sort object keys inside prototypes and examples.
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("invalid event model JSON: %w", err)
	}
	return &document, nil
}
