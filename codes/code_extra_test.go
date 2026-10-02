package codes_test

import (
	"fmt"
	"testing"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/errors"
)

// TestNewCode verifies NewCode handles both the message-present and the message-absent forms.
func TestNewCode(t *testing.T) {
	withoutMessage := codes.NewCode(7)
	if withoutMessage.Code() != 7 {
		t.Errorf("Code() = %d, want 7", withoutMessage.Code())
	}
	if withoutMessage.Message() != "" {
		t.Errorf("Message() = %q, want empty", withoutMessage.Message())
	}

	withMessage := codes.NewCode(7, "unauthorized")
	if withMessage.Code() != 7 {
		t.Errorf("Code() = %d, want 7", withMessage.Code())
	}
	if withMessage.Message() != "unauthorized" {
		t.Errorf("Message() = %q, want %q", withMessage.Message(), "unauthorized")
	}
}

// TestCodeWith verifies the copy-with helpers keep the untouched field.
func TestCodeWith(t *testing.T) {
	base := codes.NewCode(7, "unauthorized")

	if got := base.WithCode(8); got.Code() != 8 || got.Message() != "unauthorized" {
		t.Errorf("WithCode() = (%d, %q), want (8, %q)", got.Code(), got.Message(), "unauthorized")
	}
	if got := base.WithMessage("denied"); got.Code() != 7 || got.Message() != "denied" {
		t.Errorf("WithMessage() = (%d, %q), want (7, %q)", got.Code(), got.Message(), "denied")
	}
	if got := base.WithMessagef("code-%d", 9); got.Message() != "code-9" {
		t.Errorf("WithMessagef() = %q, want %q", got.Message(), "code-9")
	}
}

// TestCodeFormat verifies the %s and %v formatting of a code with and without a message.
func TestCodeFormat(t *testing.T) {
	withMessage := codes.NewCode(3, "invalid argument")
	if got := fmt.Sprintf("%s", withMessage); got != "3:invalid argument" {
		t.Errorf("%%s = %q, want %q", got, "3:invalid argument")
	}
	if got := fmt.Sprintf("%v", withMessage); got != withMessage.String() {
		t.Errorf("%%v = %q, want %q", got, withMessage.String())
	}

	withoutMessage := codes.NewCode(3)
	if got := fmt.Sprintf("%s", withoutMessage); got != "3" {
		t.Errorf("%%s = %q, want %q", got, "3")
	}
}

// TestCodeErr verifies Err returns nil for OK and a wrapping error otherwise.
func TestCodeErr(t *testing.T) {
	if err := codes.OK.Err(); err != nil {
		t.Errorf("OK.Err() = %v, want nil", err)
	}

	err := codes.InternalError.Err()
	if err == nil {
		t.Fatalf("InternalError.Err() = nil, want non-nil")
	}
	if got, want := err.Error(), codes.InternalError.String(); got != want {
		t.Errorf("Err().Error() = %q, want %q", got, want)
	}
}

// TestConvertBranches verifies every parsing branch of Convert.
func TestConvertBranches(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"nil error", nil, codes.OK.Code()},
		{"carried code", errors.NewError(codes.NotFound), codes.NotFound.Code()},
		{"plain error", errors.New("boom"), codes.Unknown.Code()},
		{"missing code field", errors.New("code error: no assignment"), codes.Unknown.Code()},
		{"missing description", errors.New("code error: code = 5"), codes.Unknown.Code()},
		{"invalid code", errors.New("code error: code = abc desc = x"), codes.Unknown.Code()},
		{"missing desc keyword", errors.New("code error: code = 5 plain"), codes.Unknown.Code()},
		{"valid text", errors.New("code error: code = 5 desc = not found"), 5},
		{"valid error type", codes.InternalError.Err(), codes.InternalError.Code()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codes.Convert(c.err); got.Code() != c.wantCode {
				t.Errorf("Convert() code = %d, want %d", got.Code(), c.wantCode)
			}
		})
	}
}
