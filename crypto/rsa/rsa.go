package rsa

type Padding string

const (
	NORMAL Padding = "NORMAL" // RSA_PKCS1_PADDING，数据切割加密长度算法为
	OAEP   Padding = "OAEP"   // RSA_PKCS1_OAEP_PADDING，数据切割加密长度算法为：公共模数长度-(2*哈希长度的)-2
)

const Name = "rsa"

// 签名填充算法
type SignPadding string

const (
	PKCS SignPadding = "PKCS" // RSA PKCS #1 v1.5
	PSS  SignPadding = "PSS"  // RSA PSS
)
