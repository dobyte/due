package errors_test

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/errors"
)

// TestNewErrorWithStack verifies NewErrorWithStack captures a stack and that %+v walks the whole
// error chain.
func TestNewErrorWithStack(t *testing.T) {
	inner := errors.NewErrorWithStack("inner", codes.InternalError, stderrors.New("base"))
	outer := errors.NewErrorWithStack("outer", codes.NotFound, inner)

	if outer.Stack() == nil {
		t.Errorf("Stack() = nil, want non-nil")
	}
	if outer.Next() != inner {
		t.Errorf("Next() = %v, want inner error", outer.Next())
	}
	if got := outer.Code(); got != codes.NotFound {
		t.Errorf("Code() = %v, want %v", got, codes.NotFound)
	}
	if outer.Error() == "" {
		t.Errorf("Error() = empty, want non-empty")
	}

	if got := fmt.Sprintf("%+v", outer); got == "" {
		t.Errorf("Sprintf(%%+v) = empty, want non-empty")
	}

	// A node without text exercises the code-based branch of the internal error helper.
	if got := fmt.Sprintf("%+v", errors.NewErrorWithStack(codes.DeadlineExceeded)); got == "" {
		t.Errorf("Sprintf(%%+v) = empty, want non-empty")
	}
}

// TestErrorHelpers verifies the package-level Code, Next, Cause, Stack and Replace helpers.
func TestErrorHelpers(t *testing.T) {
	base := stderrors.New("base")
	inner := errors.NewErrorWithStack("inner", codes.InternalError, base)
	outer := errors.NewErrorWithStack("outer", codes.NotFound, inner)
	plain := stderrors.New("plain")

	t.Run("Code", func(t *testing.T) {
		if got := errors.Code(nil); got != nil {
			t.Errorf("Code(nil) = %v, want nil", got)
		}
		if got := errors.Code(outer); got != codes.NotFound {
			t.Errorf("Code(outer) = %v, want %v", got, codes.NotFound)
		}
		if got := errors.Code(plain); got != nil {
			t.Errorf("Code(plain) = %v, want nil", got)
		}
	})

	t.Run("Next", func(t *testing.T) {
		if got := errors.Next(nil); got != nil {
			t.Errorf("Next(nil) = %v, want nil", got)
		}
		if got := errors.Next(outer); got != inner {
			t.Errorf("Next(outer) = %v, want inner", got)
		}
		if got := errors.Next(plain); got != nil {
			t.Errorf("Next(plain) = %v, want nil", got)
		}
	})

	t.Run("Cause", func(t *testing.T) {
		if got := errors.Cause(nil); got != nil {
			t.Errorf("Cause(nil) = %v, want nil", got)
		}
		if got := errors.Cause(outer); got != base {
			t.Errorf("Cause(outer) = %v, want base", got)
		}
		if got := errors.Cause(plain); got != plain {
			t.Errorf("Cause(plain) = %v, want plain", got)
		}
	})

	t.Run("Stack", func(t *testing.T) {
		if got := errors.Stack(nil); got != nil {
			t.Errorf("Stack(nil) = %v, want nil", got)
		}
		if got := errors.Stack(outer); got == nil {
			t.Errorf("Stack(outer) = nil, want non-nil")
		}
		if got := errors.Stack(plain); got != nil {
			t.Errorf("Stack(plain) = %v, want nil", got)
		}
	})

	t.Run("Replace", func(t *testing.T) {
		if got := errors.Replace(nil, "text"); got != nil {
			t.Errorf("Replace(nil) = %v, want nil", got)
		}
		if got := errors.Replace(plain, "text"); got != plain {
			t.Errorf("Replace(plain) = %v, want plain", got)
		}
	})
}

// TestErrorMethods verifies the error methods, including nil-receiver safety.
func TestErrorMethods(t *testing.T) {
	var nilErr *errors.Error

	t.Run("nil receiver", func(t *testing.T) {
		if got := nilErr.Error(); got != "" {
			t.Errorf("Error() = %q, want empty", got)
		}
		if got := nilErr.Code(); got != nil {
			t.Errorf("Code() = %v, want nil", got)
		}
		if got := nilErr.Next(); got != nil {
			t.Errorf("Next() = %v, want nil", got)
		}
		if got := nilErr.Cause(); got != nil {
			t.Errorf("Cause() = %v, want nil", got)
		}
		if got := nilErr.Stack(); got != nil {
			t.Errorf("Stack() = %v, want nil", got)
		}
		if got := nilErr.Unwrap(); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
		if got := nilErr.Replace("text"); got != nil {
			t.Errorf("Replace() = %v, want nil", got)
		}
		if got := nilErr.String(); got != "" {
			t.Errorf("String() = %q, want empty", got)
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		base := stderrors.New("base")
		err := errors.NewError("wrapped", base)
		if got := err.Unwrap(); got != base {
			t.Errorf("Unwrap() = %v, want base", got)
		}
	})

	t.Run("Cause returns self without underlying error", func(t *testing.T) {
		err := errors.NewError("self")
		if got := err.Cause(); got != err {
			t.Errorf("Cause() = %v, want self", got)
		}
	})

	t.Run("Replace", func(t *testing.T) {
		err := errors.NewError(codes.NotFound, "original")
		if got := err.Replace("updated"); got != err {
			t.Errorf("Replace() = %v, want self", got)
		}
		if got := err.Error(); got != codes.NotFound.String()+": updated" {
			t.Errorf("Replace() text = %q, want updated", got)
		}

		mismatch := errors.NewError(codes.NotFound, "original")
		mismatch.Replace("updated", codes.InternalError)
		if got := mismatch.Error(); got != codes.NotFound.String()+": original" {
			t.Errorf("Replace() with mismatched condition = %q, want unchanged", got)
		}

		match := errors.NewError(codes.NotFound, "original")
		match.Replace("updated", codes.NotFound)
		if got := match.Error(); got != codes.NotFound.String()+": updated" {
			t.Errorf("Replace() with matched condition = %q, want updated", got)
		}
	})

	t.Run("String", func(t *testing.T) {
		err := errors.NewErrorWithStack("outer", codes.NotFound)
		if got := err.String(); got == "" {
			t.Errorf("String() = empty, want non-empty")
		}
	})
}

// TestErrorText verifies the composition rules of Error.
func TestErrorText(t *testing.T) {
	base := stderrors.New("boom")

	cases := []struct {
		name string
		err  *errors.Error
		want string
	}{
		{"empty", errors.NewError(), ""},
		{"code only", errors.NewError(codes.NotFound), codes.NotFound.String()},
		{"text only", errors.NewError("just text"), "just text"},
		{"err only", errors.NewError(base), "boom"},
		{"ok code with text", errors.NewError(codes.OK, "just text"), "just text"},
		{"code and text", errors.NewError(codes.NotFound, "detail"), codes.NotFound.String() + ": detail"},
		{"code and err", errors.NewError(codes.NotFound, base), codes.NotFound.String() + ": boom"},
		{"text and err", errors.NewError("context", base), "context: boom"},
		{"code text and err", errors.NewError(codes.NotFound, "context", base), codes.NotFound.String() + ": context: boom"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestErrorFormat verifies the supported Format verbs.
func TestErrorFormat(t *testing.T) {
	t.Run("s with text", func(t *testing.T) {
		err := errors.NewError(codes.NotFound, "custom")
		if got := fmt.Sprintf("%s", err); got != "custom" {
			t.Errorf("%%s = %q, want %q", got, "custom")
		}
	})

	t.Run("s with code", func(t *testing.T) {
		err := errors.NewError(codes.NotFound)
		if got := fmt.Sprintf("%s", err); got != "5:not found" {
			t.Errorf("%%s = %q, want %q", got, "5:not found")
		}
	})

	t.Run("v without plus", func(t *testing.T) {
		err := errors.NewError(codes.NotFound, "custom")
		if got := fmt.Sprintf("%v", err); got != err.Error() {
			t.Errorf("%%v = %q, want %q", got, err.Error())
		}
	})
}

// TestGo113Wrappers verifies the standard-library error wrappers.
func TestGo113Wrappers(t *testing.T) {
	base := stderrors.New("base")
	wrapped := errors.NewError("wrapped", base)

	t.Run("Is", func(t *testing.T) {
		if !errors.Is(wrapped, base) {
			t.Errorf("Is(wrapped, base) = false, want true")
		}
		if errors.Is(base, wrapped) {
			t.Errorf("Is(base, wrapped) = true, want false")
		}
	})

	t.Run("As", func(t *testing.T) {
		var target *errors.Error
		if !errors.As(wrapped, &target) {
			t.Fatalf("As(wrapped, &target) = false, want true")
		}
		if target != wrapped {
			t.Errorf("As() target = %v, want wrapped", target)
		}
	})

	t.Run("Unwrap", func(t *testing.T) {
		if got := errors.Unwrap(wrapped); got != base {
			t.Errorf("Unwrap() = %v, want base", got)
		}
		if got := errors.Unwrap(base); got != nil {
			t.Errorf("Unwrap() = %v, want nil", got)
		}
	})
}
