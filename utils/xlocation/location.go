package xlocation

import (
	"context"

	"github.com/dobyte/due/v2/core/location"
)

var globalLocation = location.NewLocation(
	location.NewIPWHOISResolver(),
	location.NewIPAPICOResolver(),
	location.NewIPAPICOMResolver(),
)

// Parse resolves the given IP address.
//
// The ctx is used for timeout control.
func Parse(ctx context.Context, ip string) (*location.Result, error) {
	return globalLocation.Parse(ctx, ip)
}
