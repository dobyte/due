package crypto

type Signer interface {
	// Sign 签名
	Sign(data []byte) ([]byte, error)
	// Verify 验签
	Verify(data []byte, signature []byte) error
}
