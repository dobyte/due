package http

import (
	"github.com/dobyte/due/v2/codes"
	"github.com/gofiber/fiber/v3"
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
