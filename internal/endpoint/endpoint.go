package endpoint

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	secureField = "is_secure"
)

type Endpoint struct {
	raw      *url.URL
	isSecure bool
}

func ParseEndpoint(endpoint string) (*Endpoint, error) {
	raw, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &Endpoint{raw: raw, isSecure: raw.Query().Get(secureField) == "true"}, nil
}

func NewEndpoint(scheme, address string, isSecure bool) *Endpoint {
	return &Endpoint{
		raw: &url.URL{
			Scheme:   scheme,
			Host:     address,
			RawQuery: fmt.Sprintf("%s=%s", secureField, strconv.FormatBool(isSecure)),
		},
		isSecure: isSecure,
	}
}

func (e *Endpoint) Scheme() string {
	return e.raw.Scheme
}

func (e *Endpoint) Target() string {
	return "direct://" + e.raw.Host
}

func (e *Endpoint) Address() string {
	return e.raw.Host
}

func (e *Endpoint) IsSecure() bool {
	return e.isSecure
}

func (e *Endpoint) String() string {
	return e.raw.String()
}
