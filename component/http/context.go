package http

import (
	"bytes"
	"github.com/dobyte/due/v2/codes"
	"github.com/gofiber/fiber/v3"
	"io"
	"net/http"
	"net/url"
)

type Resp struct {
	Code int `json:"code"`           // 响应码
	Data any `json:"data,omitempty"` // 响应数据
}

type Context interface {
	fiber.Ctx
	// CTX 获取fiber.Ctx
	CTX() fiber.Ctx
	// Proxy 获取代理API
	Proxy() *Proxy
	// Failure 失败响应
	Failure(rst any) error
	// Success 成功响应
	Success(data ...any) error
	// StdRequest 获取标准请求（net/http）
	StdRequest() *http.Request
}

type context struct {
	fiber.Ctx
	proxy *Proxy
}

// CTX 获取fiber.Ctx
func (c *context) CTX() fiber.Ctx {
	return c.Ctx
}

// Proxy 代理API
func (c *context) Proxy() *Proxy {
	return c.proxy
}

// Failure 失败响应
func (c *context) Failure(rst any) error {
	switch v := rst.(type) {
	case error:
		return c.JSON(&Resp{Code: codes.Convert(v).Code()})
	case *codes.Code:
		return c.JSON(&Resp{Code: v.Code()})
	default:
		return c.JSON(&Resp{Code: codes.Unknown.Code()})
	}
}

// Success 成功响应
func (c *context) Success(data ...any) error {
	if len(data) > 0 {
		return c.JSON(&Resp{Code: codes.OK.Code(), Data: data[0]})
	} else {
		return c.JSON(&Resp{Code: codes.OK.Code()})
	}
}

// StdRequest 获取标准请求（net/http）
func (c *context) StdRequest() *http.Request {
	req := c.Request()

	std := &http.Request{}
	std.Method = c.Method()
	std.URL, _ = url.Parse(req.URI().String())
	std.Proto = c.Protocol()
	std.ProtoMajor, std.ProtoMinor, _ = http.ParseHTTPVersion(std.Proto)
	std.Header = c.GetReqHeaders()
	std.Host = c.Host()
	std.ContentLength = int64(len(c.Body()))
	std.RemoteAddr = c.Context().RemoteAddr().String()
	std.RequestURI = string(req.RequestURI())

	if req.Body() != nil {
		std.Body = io.NopCloser(bytes.NewReader(req.Body()))
	}

	return std
}
