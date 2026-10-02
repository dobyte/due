package location

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
)

const (
	maxResponseSize = 1 << 20 // Maximum number of response body bytes to read (1MB)
)

// Resolver is the IP address resolver interface.
type Resolver interface {
	// Name returns the resolver name.
	Name() string
	// Resolve resolves the geolocation of ip.
	Resolve(ctx context.Context, ip string) (*Result, error)
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

// fetchJSON performs an HTTP GET request to url and decodes the JSON response into out.
func fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return errors.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return err
	}

	if len(data) > maxResponseSize {
		return errors.New(fmt.Sprintf("response body too large: %d bytes", len(data)))
	}

	return json.Unmarshal(data, out)
}

// normalizeISP converts the ISP name to its Chinese name, returning isp unchanged when it is not
// recognized.
func normalizeISP(isp string) string {
	lower := strings.ToLower(isp)
	switch {
	case strings.Contains(lower, "china mobile"), strings.Contains(lower, "cmnet"):
		return "移动"
	case strings.Contains(lower, "china unicom"), strings.Contains(lower, "unicom"), strings.Contains(lower, "cncgroup"):
		return "联通"
	case strings.Contains(lower, "china telecom"), strings.Contains(lower, "chinanet"):
		return "电信"
	case strings.Contains(lower, "china broadcasting"), strings.Contains(lower, "cbn"):
		return "广电"
	case strings.Contains(lower, "tietong"), strings.Contains(lower, "crtc"):
		return "铁通"
	case strings.Contains(lower, "cernet"), strings.Contains(lower, "education"):
		return "教育网"
	default:
		return isp
	}
}
