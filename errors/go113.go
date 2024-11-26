package errors

import (
	"errors"
)

// New Wrapping for errors.New standard library
func New(text string) error { return errors.New(text) }

// Is Wrapping for errors.Is standard library
func Is(err, target error) bool { return errors.Is(err, target) }

// As Wrapping for errors.As standard library
func As(err error, target any) bool { return errors.As(err, target) }

// Unwrap Wrapping for errors.Unwrap standard library
func Unwrap(err error) error { return errors.Unwrap(err) }
