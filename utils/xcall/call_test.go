package xcall_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xcall"
)

func TestBackoff(t *testing.T) {
	err := xcall.Backoff(context.Background(), func(attempt int) (bool, error) {
		fmt.Printf("attempt: %d\n", attempt)

		return true, errors.New("backoff test error")
	}, 5, 100*time.Millisecond, 1000*time.Millisecond)

	t.Logf("err: %v", err)
}
