package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type Resp struct {
	Code    int    `json:"code"`              // Response code
	Message string `json:"message"`           // Response message
	Details string `json:"details,omitempty"` // Response details
	Data    any    `json:"data,omitempty"`    // Response data
}

// Context is the HTTP context interface.
//
// It extends [fiber.Ctx] with a proxy API, response handling and a standard request.
type Context interface {
	fiber.Ctx
	// CTX returns the underlying [fiber.Ctx].
	CTX() fiber.Ctx
	// Proxy returns the proxy API.
	Proxy() *Proxy
	// Failure writes a failure response.
	Failure(rst any) error
	// Success writes a success response.
	Success(data ...any) error
	// StdRequest returns the equivalent [http.Request] (net/http).
	StdRequest() *http.Request
}

// context is the HTTP context.
type context struct {
	*fiber.DefaultCtx
	proxy      *Proxy
	stdRequest *http.Request
}

// newContext creates an HTTP context from the fiber context ctx and the proxy.
func newContext(ctx *fiber.DefaultCtx, proxy *Proxy) *context {
	return &context{
		DefaultCtx: ctx,
		proxy:      proxy,
	}
}

// CTX returns the underlying [fiber.Ctx].
func (c *context) CTX() fiber.Ctx {
	return c
}

// Proxy returns the proxy API.
func (c *context) Proxy() *Proxy {
	return c.proxy
}

// Failure writes a failure response.
//
// It converts rst, which may be an error, a [codes.Code] or a *[codes.Code], into the matching
// HTTP error response. It returns an error when writing the response fails.
func (c *context) Failure(rst any) error {
	switch v := rst.(type) {
	case error:
		if code := codes.Convert(v); code != nil {
			message := code.Message()

			switch parts := strings.SplitN(message, ": ", 2); len(parts) {
			case 2:
				if mode.IsReleaseMode() {
					return c.JSON(&Resp{Code: code.Code(), Message: parts[0]})
				} else {
					return c.JSON(&Resp{Code: code.Code(), Message: parts[0], Details: parts[1]})
				}
			default:
				return c.JSON(&Resp{Code: code.Code(), Message: message})
			}
		}
	case codes.Code:
		return c.JSON(&Resp{Code: v.Code(), Message: v.Message()})
	case *codes.Code:
		if v != nil {
			return c.JSON(&Resp{Code: v.Code(), Message: v.Message()})
		}
	}

	return c.JSON(&Resp{Code: codes.Unknown.Code(), Message: codes.Unknown.Message()})
}

// Success writes a success response.
func (c *context) Success(data ...any) error {
	if len(data) > 0 {
		return c.JSON(&Resp{Code: codes.OK.Code(), Message: codes.OK.Message(), Data: data[0]})
	} else {
		return c.JSON(&Resp{Code: codes.OK.Code(), Message: codes.OK.Message()})
	}
}

// Reset resets the context.
func (c *context) Reset(fctx *fasthttp.RequestCtx) {
	c.DefaultCtx.Reset(fctx)
	c.stdRequest = nil
}

// StdRequest returns the equivalent [http.Request] (net/http).
//
// Note that the returned request body is copied into an independent buffer, so it stays safe to
// use after the handler returns.
func (c *context) StdRequest() *http.Request {
	if c.stdRequest != nil {
		return c.stdRequest
	}

	c.stdRequest = &http.Request{}

	if err := fasthttpadaptor.ConvertRequest(c.RequestCtx(), c.stdRequest, true); err != nil {
		log.Errorf("convert request failed: %v", err)
	}

	// Copy the request body to avoid referencing fasthttp's pooled memory, which is overwritten
	// once the connection is reused.
	if c.stdRequest.Body != nil {
		if body, err := io.ReadAll(c.stdRequest.Body); err != nil {
			log.Errorf("copy request body failed: %v", err)
		} else {
			c.stdRequest.Body = io.NopCloser(bytes.NewReader(body))
			c.stdRequest.ContentLength = int64(len(body))
		}
	}

	return c.stdRequest
}
