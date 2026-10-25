package gate

import (
	"testing"

	"github.com/dobyte/due/v2/errors"
)

func TestUnbindConnPreservesReplacement(t *testing.T) {
	s := newSession()
	first, second := newStubConn(1), newStubConn(2)
	s.addConn(first)
	s.addConn(second)
	if err := s.bind(1, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.bind(2, 100); err != nil {
		t.Fatal(err)
	}
	if err := s.unbindConn(1, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("stale unbind returned %v", err)
	}
	if !s.isBound(2, 100) || s.isBound(1, 100) {
		t.Fatal("stale unbind changed the replacement binding")
	}
	if err := s.unbindConn(0, 100); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("zero CID unbound a user: %v", err)
	}
	if err := s.unbindConn(2, 100); err != nil {
		t.Fatal(err)
	}
	if uid, err := s.userID(2); err != nil || uid != 0 {
		t.Fatalf("unbound connection has UID %d, err %v", uid, err)
	}
	if _, err := s.userID(404); !errors.Is(err, errors.ErrNotFoundSession) {
		t.Fatalf("missing connection returned %v", err)
	}
}
