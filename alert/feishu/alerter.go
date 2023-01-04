package feishu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/dobyte/due/alert"
	"github.com/dobyte/due/errors"
	"github.com/dobyte/due/utils/xtime"
	"io"
	"net/http"
	"strconv"
)

const Name = "feishu"

var _ alert.Alerter = &Alerter{}

func init() {
	alert.Register(NewAlerter())
}

type Alerter struct {
	opts *options
}

func NewAlerter(opts ...Option) *Alerter {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	return &Alerter{opts: o}
}

// Name 名称
func (a *Alerter) Name() string {
	return Name
}

// Alert 报警
func (a *Alerter) Alert(msg string) error {
	return a.doRequest("text", &textContent{
		Text: msg,
	})
}

// 发起请求
func (a *Alerter) doRequest(msgType msgType, Content interface{}) error {
	req := &request{
		MsgType: msgType,
		Content: Content,
	}

	if a.opts.secret != "" {
		req.Timestamp = strconv.FormatInt(xtime.Now().Unix(), 10)

		var data []byte
		h := hmac.New(sha256.New, []byte(req.Timestamp+"\n"+a.opts.secret))
		if _, err := h.Write(data); err != nil {
			return err
		}
		req.Sign = base64.StdEncoding.EncodeToString(h.Sum(nil))
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	res, err := http.Post(a.opts.webhook, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return errors.New(res.Status)
	}

	if data, err = io.ReadAll(res.Body); err != nil {
		return err
	}

	_ = res.Body.Close()

	resp := &response{}
	if err = json.Unmarshal(data, &resp); err != nil {
		return err
	}

	if resp.Code != 0 {
		return errors.New(resp.Msg)
	}

	return nil
}
