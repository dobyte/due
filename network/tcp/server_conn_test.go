package tcp

import (
	"testing"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
)

// TestServerConnCloseIfIgnoresMismatchedID verifies that a stale reference does not close the
// connection that reuses the object: the ID check must run before the close claim.
func TestServerConnCloseIfIgnoresMismatchedID(t *testing.T) {
	c := &serverConn{id: 7}
	c.state.Store(int32(network.ConnOpened))

	if err := c.CloseIf(8); !errors.Is(err, errors.ErrConnectionClosed) {
		t.Fatalf("unexpected error: %v", err)
	}

	if state := c.State(); state != network.ConnOpened {
		t.Fatalf("connection state changed to %v", state)
	}
}
