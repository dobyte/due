package quic

import "time"

const (
	protocol            = "quic"
	alpn                = "due-quic"
	defaultCloseTimeout = 5 * time.Second
)
