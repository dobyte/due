package http

import (
	"github.com/dobyte/due/v2/codes"
	"github.com/gofiber/fiber/v3"
)

const RespPanic = "http response panic"

type body struct {
	Code int `json:"code"`
	Data any `json:"data,omitempty"`
}

type Context interface {
	fiber.Ctx
	// Proxy 获取代理API
	Proxy() *Proxy
	// Fail 失败响应
	Fail(rst any) error
	// Success 成功响应
	Success(data ...any) error
}

type context struct {
	fiber.Ctx
	proxy *Proxy
}

// Proxy 代理API
func (c *context) Proxy() *Proxy {
	return c.proxy
}

// Fail 失败响应
func (c *context) Fail(rst any) error {
	switch v := rst.(type) {
	case error:
		return c.JSON(&body{Code: codes.Convert(v).Code()})
	case *codes.Code:
		return c.JSON(&body{Code: v.Code()})
	default:
		return c.JSON(&body{Code: codes.Unknown.Code()})
	}
}

// Success 成功响应
func (c *context) Success(data ...any) error {
	if len(data) > 0 {
		return c.JSON(&body{Code: codes.OK.Code(), Data: data[0]})
	} else {
		return c.JSON(&body{Code: codes.OK.Code()})
	}
}
