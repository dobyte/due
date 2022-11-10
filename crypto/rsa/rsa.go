package rsa

const Name = "rsa"

// EncryptPadding 加密填充算法
type EncryptPadding string

const (
	NORMAL EncryptPadding = "NORMAL" // RSA_PKCS1_PADDING，数据切割加密长度算法为
	OAEP   EncryptPadding = "OAEP"   // RSA_PKCS1_OAEP_PADDING，数据切割加密长度算法为：公共模数长度-(2*哈希长度的)-2
)

// SignPadding 签名填充算法
type SignPadding string

const (
	PKCS SignPadding = "PKCS" // RSA PKCS #1 v1.5
	PSS  SignPadding = "PSS"  // RSA PSS
)
