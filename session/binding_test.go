package session

import (
	"testing"

	"github.com/dobyte/due/v2/errors"
)

func TestUnbindConnPreservesReplacement(t *testing.T) {
	s := NewSession()
	first, second := newStubConn(1), newStubConn(2)
	s.AddConn(first)
	s.AddConn(second)
	if err := s.Bind(1, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.Bind(2, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.UnbindConn(1, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("stale unbind returned %v", err)
	}
	if !s.IsBound(2, 100) || s.IsBound(1, 100) {
		t.Fatal("stale unbind changed the replacement binding")
	}
	if err := s.UnbindConn(0, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("zero CID unbound a user: %v", err)
	}
	if err := s.UnbindConn(2, 100); err != nil {
		t.Fatal(err)
	}
	if uid, err := s.UserID(2); err != nil || uid != 0 {
		t.Fatalf("unbound connection has UID %d, err %v", uid, err)
	}
	if _, err := s.UserID(404); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("missing connection returned %v", err)
	}
}
