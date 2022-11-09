package crypto

type Signer interface {
	// Name 名称
	Name() string
	// Sign 签名
	Sign(data []byte) ([]byte, error)
}
