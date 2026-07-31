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
	xcall.Backoff(context.Background(), func(attempt int) error {
		fmt.Printf("attempt: %d\n", attempt)

		if attempt == 5 {
			return nil
		}

		return errors.New("backoff test error")
	}, 5, 100*time.Millisecond, 1000*time.Millisecond)
}
