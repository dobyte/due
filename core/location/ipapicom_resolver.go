package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPAPICOMResolver is an IP address resolver based on the ip-api.com service.
type IPAPICOMResolver struct {
}

var _ Resolver = (*IPAPICOMResolver)(nil)

// NewIPAPICOMResolver returns an IP address resolver based on the ip-api.com service.
func NewIPAPICOMResolver() *IPAPICOMResolver {
	return &IPAPICOMResolver{}
}

// Name returns the resolver name.
func (i *IPAPICOMResolver) Name() string {
	return "ip-api.com"
}

// Resolve resolves the geolocation of ip.
func (i *IPAPICOMResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
		Query      string `json:"query"`
	}

	endpoint := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,message,country,regionName,city,isp,query", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		if resp.Message != "" {
			return nil, errors.New(resp.Message)
		}

		return nil, errors.New("query failed")
	}

	return &Result{
		IP:       resp.Query,
		Country:  resp.Country,
		Province: resp.RegionName,
		City:     resp.City,
		ISP:      normalizeISP(resp.ISP),
	}, nil
}
