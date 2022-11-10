package crypto

type Encryptor interface {
	// Name 名称
	Name() string
	// Encrypt 加密
	Encrypt(data []byte) ([]byte, error)
}
