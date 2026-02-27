package cmd

import (
	"encoding/json"
	"fmt"
	"io"
)

// StreamEncoder handles streaming output for secrets export.
type StreamEncoder interface {
	Start() error
	StartClient(name string) error
	StartNamespace(name string) error
	WriteSecret(key, value string) error
	EndNamespace() error
	EndClient() error
	End() error
}

// JSONStreamEncoder implements StreamEncoder for JSON output.
type JSONStreamEncoder struct {
	w              io.Writer
	enc            *json.Encoder
	firstClient    bool
	firstNamespace bool
	firstSecret    bool
}

func NewJSONStreamEncoder(w io.Writer) *JSONStreamEncoder {
	return &JSONStreamEncoder{
		w:           w,
		enc:         json.NewEncoder(w),
		firstClient: true,
	}
}

func (e *JSONStreamEncoder) Start() error {
	_, err := fmt.Fprint(e.w, "{\n")
	return err
}

func (e *JSONStreamEncoder) StartClient(name string) error {
	if !e.firstClient {
		if _, err := fmt.Fprint(e.w, ",\n"); err != nil {
			return err
		}
	}
	e.firstClient = false
	if _, err := fmt.Fprintf(e.w, "  %q: {\n", name); err != nil {
		return err
	}
	e.firstNamespace = true
	return nil
}

func (e *JSONStreamEncoder) StartNamespace(name string) error {
	if !e.firstNamespace {
		if _, err := fmt.Fprint(e.w, ",\n"); err != nil {
			return err
		}
	}
	e.firstNamespace = false
	if _, err := fmt.Fprintf(e.w, "    %q: {\n", name); err != nil {
		return err
	}
	e.firstSecret = true
	return nil
}

func (e *JSONStreamEncoder) WriteSecret(key, value string) error {
	if !e.firstSecret {
		if _, err := fmt.Fprint(e.w, ",\n"); err != nil {
			return err
		}
	}
	e.firstSecret = false
	// We construct a temporary map to marshal the key-value pair correctly with escaping
	// getting the escaping right manually is hard, but we can marshal just the value
	valBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	keyBytes, err := json.Marshal(key)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "      %s: %s", string(keyBytes), string(valBytes))
	return err
}

func (e *JSONStreamEncoder) EndNamespace() error {
	_, err := fmt.Fprint(e.w, "\n    }")
	return err
}

func (e *JSONStreamEncoder) EndClient() error {
	_, err := fmt.Fprint(e.w, "\n  }")
	return err
}

func (e *JSONStreamEncoder) End() error {
	_, err := fmt.Fprint(e.w, "\n}\n")
	return err
}

// YAMLStreamEncoder implements StreamEncoder for YAML output.
type YAMLStreamEncoder struct {
	w io.Writer
}

func NewYAMLStreamEncoder(w io.Writer) *YAMLStreamEncoder {
	return &YAMLStreamEncoder{w: w}
}

func (e *YAMLStreamEncoder) Start() error { return nil }

func (e *YAMLStreamEncoder) StartClient(name string) error {
	_, err := fmt.Fprintf(e.w, "%s:\n", name)
	return err
}

func (e *YAMLStreamEncoder) StartNamespace(name string) error {
	_, err := fmt.Fprintf(e.w, "  %s:\n", name)
	return err
}

func (e *YAMLStreamEncoder) WriteSecret(key, value string) error {
	escapedValue := value
	if containsSpecialChars(value) {
		escapedValue = fmt.Sprintf("%q", value)
	}
	_, err := fmt.Fprintf(e.w, "    %s: %s\n", key, escapedValue)
	return err
}

func (e *YAMLStreamEncoder) EndNamespace() error { return nil }
func (e *YAMLStreamEncoder) EndClient() error    { return nil }
func (e *YAMLStreamEncoder) End() error          { return nil }

// containsSpecialChars checks if a string contains special YAML characters
// Copied from secrets.go, can be refactored to be shared or just duplicated for simplicity here
func containsSpecialChars(s string) bool {
	specialChars := []rune{':', '#', '\'', '"', '\n', '\t'}
	for _, char := range s {
		for _, special := range specialChars {
			if char == special {
				return true
			}
		}
	}
	return false
}
