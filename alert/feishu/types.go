package feishu

type msgType string

type request struct {
	Timestamp string      `json:"timestamp,omitempty"`
	Sign      string      `json:"sign,omitempty"`
	MsgType   msgType     `json:"msg_type"`
	Content   interface{} `json:"content"`
}

type response struct {
	Code int    `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
}

type textContent struct {
	Text string `json:"text"`
}
