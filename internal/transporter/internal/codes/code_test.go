package codes

import (
	"testing"

	"github.com/dobyte/due/v2/errors"
)

// TestErrorToCode verifies the error-to-code mapping.
func TestErrorToCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want uint16
	}{
		{"nil", nil, OK},
		{"not found session", errors.ErrNotFoundSession, NotFoundSession},
		{"other", errors.ErrInvalidArgument, InternalError},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if got := ErrorToCode(c.err); got != c.want {
				t.Errorf("ErrorToCode(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

// TestCodeToError verifies the code-to-error mapping.
func TestCodeToError(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		want error
	}{
		{"ok", OK, nil},
		{"not found session", NotFoundSession, errors.ErrNotFoundSession},
		{"unknown", InternalError, errors.ErrUnknownError},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			if got := CodeToError(c.code); !errors.Is(got, c.want) {
				t.Errorf("CodeToError(%d) = %v, want %v", c.code, got, c.want)
			}
		})
	}
}
