package crypto

type Decryptor interface {
	// Name 名称
	Name() string
	// Decrypt 解密
	Decrypt(data []byte) ([]byte, error)
}
