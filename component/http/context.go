package http

import (
	"bytes"
	"io"
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

// Context HTTP上下文接口
// 扩展fiber.Ctx，提供代理API、响应处理及标准请求等能力
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

// HTTP上下文
type context struct {
	*fiber.DefaultCtx
	proxy          *Proxy
	stdRequest     *http.Request
	stdRequestOnce *sync.Once
}

// 创建HTTP上下文
// @param ctx *fiber.DefaultCtx fiber默认上下文
// @param proxy *Proxy HTTP代理
// @return @1 *context HTTP上下文
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
// 根据响应内容的类型转换为对应的HTTP错误响应
// @param rst any 响应内容，支持error、codes.Code及*codes.Code
// @return @1 error 写入响应失败时返回的错误
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
// 注意：返回的请求体已拷贝为独立内存，可在处理器返回后安全使用
// @return @1 *http.Request 标准请求
func (c *context) StdRequest() *http.Request {
	c.stdRequestOnce.Do(func() {
		if c.stdRequest == nil {
			c.stdRequest = &http.Request{}
		}

		if err := fasthttpadaptor.ConvertRequest(c.RequestCtx(), c.stdRequest, true); err != nil {
			log.Errorf("convert request failed: %v", err)
		}

		// 拷贝请求体，避免引用fasthttp请求池内存（连接复用后会被覆盖）
		if c.stdRequest.Body != nil {
			body, err := io.ReadAll(c.stdRequest.Body)
			if err != nil {
				log.Errorf("copy request body failed: %v", err)
			} else {
				c.stdRequest.Body = io.NopCloser(bytes.NewReader(body))
				c.stdRequest.ContentLength = int64(len(body))
			}
		}
	})

	return c.stdRequest
}
