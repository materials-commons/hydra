package decoder

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func DecodeMapStrict[T any](m map[string]any) (T, error) {
	var out T

	b, err := json.Marshal(m)
	if err != nil {
		return out, fmt.Errorf("failed to marshal map: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&out); err != nil {
		return out, fmt.Errorf("failed to decode map: %w", err)
	}

	return out, nil
}
