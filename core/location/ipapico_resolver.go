package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPAPICOResolver is an IP address resolver based on the ipapi.co service.
type IPAPICOResolver struct {
}

var _ Resolver = (*IPAPICOResolver)(nil)

// NewIPAPICOResolver returns an IP address resolver based on the ipapi.co service.
func NewIPAPICOResolver() *IPAPICOResolver {
	return &IPAPICOResolver{}
}

// Name returns the resolver name.
func (i *IPAPICOResolver) Name() string {
	return "ipapi.co"
}

// Resolve resolves the geolocation of ip.
func (i *IPAPICOResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		IP      string `json:"ip"`
		Country string `json:"country_name"`
		Region  string `json:"region"`
		City    string `json:"city"`
		Org     string `json:"org"`
		Error   bool   `json:"error"`
		Reason  string `json:"reason"`
	}

	endpoint := fmt.Sprintf("https://ipapi.co/%s/json/", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		if resp.Reason != "" {
			return nil, errors.New(resp.Reason)
		}

		return nil, errors.New("query failed")
	}

	return &Result{
		IP:       resp.IP,
		Country:  resp.Country,
		Province: resp.Region,
		City:     resp.City,
		ISP:      normalizeISP(resp.Org),
	}, nil
}
