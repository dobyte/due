package crypto

type Verifier interface {
	// Name 名称
	Name() string
	// Verify 验签
	Verify(data []byte, signature string) error
}
