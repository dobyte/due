package http

import (
	"net/http"
	"strings"
	"sync"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type Resp struct {
	Code    int    `json:"code"`              // 响应码
	Message string `json:"message"`           // 响应消息
	Details string `json:"details,omitempty"` // 响应详情
	Data    any    `json:"data,omitempty"`    // 响应数据
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
	*fiber.DefaultCtx
	proxy          *Proxy
	stdRequest     *http.Request
	stdRequestOnce *sync.Once
}

func newContext(ctx *fiber.DefaultCtx, proxy *Proxy) *context {
	return &context{
		DefaultCtx: ctx,
		proxy:      proxy,
	}
}

// CTX 获取fiber.Ctx
func (c *context) CTX() fiber.Ctx {
	return c
}

// Proxy 代理API
func (c *context) Proxy() *Proxy {
	return c.proxy
}

// Failure 失败响应
func (c *context) Failure(rst any) error {
	switch v := rst.(type) {
	case error:
		code := codes.Convert(v)
		message := code.Message()

		switch parts := strings.SplitN(message, ": ", 2); len(parts) {
		case 2:
			if mode.IsReleaseMode() {
				return c.JSON(&Resp{Code: code.Code(), Message: parts[0]})
			} else {
				return c.JSON(&Resp{Code: code.Code(), Message: parts[0], Details: parts[1]})
			}
		case 1:
			return c.JSON(&Resp{Code: code.Code(), Message: parts[0]})
		default:
			return c.JSON(&Resp{Code: code.Code(), Message: message})
		}
	case codes.Code:
		return c.JSON(&Resp{Code: v.Code(), Message: v.Message()})
	case *codes.Code:
		return c.JSON(&Resp{Code: v.Code(), Message: v.Message()})
	default:
		return c.JSON(&Resp{Code: codes.Unknown.Code(), Message: codes.Unknown.Message()})
	}
}

// Success 成功响应
func (c *context) Success(data ...any) error {
	if len(data) > 0 {
		return c.JSON(&Resp{Code: codes.OK.Code(), Message: codes.OK.Message(), Data: data[0]})
	} else {
		return c.JSON(&Resp{Code: codes.OK.Code(), Message: codes.OK.Message()})
	}
}

// Reset 重置上下文
func (c *context) Reset(fctx *fasthttp.RequestCtx) {
	c.DefaultCtx.Reset(fctx)
	c.stdRequest = nil
	c.stdRequestOnce = &sync.Once{}
}

// StdRequest 获取标准请求（net/http）
func (c *context) StdRequest() *http.Request {
	c.stdRequestOnce.Do(func() {
		if c.stdRequest == nil {
			c.stdRequest = &http.Request{}
		}

		if err := fasthttpadaptor.ConvertRequest(c.RequestCtx(), c.stdRequest, true); err != nil {
			log.Errorf("convert request failed: %v", err)
		}
	})

	return c.stdRequest
}
