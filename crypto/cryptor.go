package crypto

type Cryptor interface {
	// Encrypt 加密
	Encrypt(data []byte) ([]byte, error)
	// Decrypt 解密
	Decrypt(data []byte) ([]byte, error)
}
