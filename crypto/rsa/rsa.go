package rsa

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/pem"
	"github.com/dobyte/due/utils/xconv"
	"github.com/dobyte/due/utils/xpath"
	"hash"
	"io/ioutil"
)

type Padding string

const (
	NORMAL Padding = "NORMAL" // RSA_PKCS1_PADDING，数据切割加密长度算法为
	OAEP   Padding = "OAEP"   // RSA_PKCS1_OAEP_PADDING，数据切割加密长度算法为：公共模数长度-(2*哈希长度的)-2
)

type Hash string

const (
	SHA1   Hash = "sha1"   // 长度为 sha1.Size
	SHA224 Hash = "sha224" // 长度为 sha256.Size224
	SHA256 Hash = "sha256" // 长度为 sha256.Size
	SHA384 Hash = "sha384" // 长度为 sha512.Size384
	SHA512 Hash = "sha512" // 长度为 sha256.Size
)

func (h Hash) New() hash.Hash {
	switch h {
	case SHA1:
		return sha1.New()
	case SHA224:
		return sha256.New224()
	case SHA256:
		return sha256.New()
	case SHA384:
		return sha512.New384()
	case SHA512:
		return sha512.New()
	default:
		return sha256.New()
	}
}

func (h Hash) Size() int {
	switch h {
	case SHA1:
		return sha1.Size
	case SHA224:
		return sha256.Size224
	case SHA256:
		return sha256.Size
	case SHA384:
		return sha512.Size384
	case SHA512:
		return sha512.Size
	default:
		return sha256.Size
	}
}

func loadKey(key string) (*pem.Block, error) {
	isFile, err := xpath.IsFile(key)
	if err != nil {
		return nil, err
	}

	var buffer []byte

	if isFile {
		buffer, err = ioutil.ReadFile(key)
		if err != nil {
			return nil, err
		}
	} else {
		buffer = xconv.StringToBytes(key)
	}

	block, _ := pem.Decode(buffer)

	return block, nil
}
