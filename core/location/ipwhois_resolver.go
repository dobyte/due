package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPWHOISResolver is an IP address resolver based on the ipwho.is service.
type IPWHOISResolver struct {
}

var _ Resolver = (*IPWHOISResolver)(nil)

// NewIPWHOISResolver returns an IP address resolver based on the ipwho.is service.
func NewIPWHOISResolver() *IPWHOISResolver {
	return &IPWHOISResolver{}
}

// Name returns the resolver name.
func (i *IPWHOISResolver) Name() string {
	return "ipwho.is"
}

// Resolve resolves the geolocation of ip.
func (i *IPWHOISResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		Success    bool   `json:"success"`
		Message    string `json:"message"`
		IP         string `json:"ip"`
		Country    string `json:"country"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}

	endpoint := fmt.Sprintf("https://ipwho.is/%s?lang=zh-CN", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Success {
		return &Result{
			IP:       resp.IP,
			Country:  resp.Country,
			Province: resp.Region,
			City:     resp.City,
			ISP:      normalizeISP(resp.Connection.ISP),
		}, nil
	}

	if resp.Message != "" {
		return nil, errors.New(resp.Message)
	}

	return nil, errors.New("query failed")
}
