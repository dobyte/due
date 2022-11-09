/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/11/1 1:23 上午
 * @Desc: TODO
 */

package hash

import (
	"crypto"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
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

func (h Hash) Sum(data []byte) []byte {
	switch h {
	case SHA1:
		sum := sha1.Sum(data)
		return sum[:]
	case SHA224:
		sum := sha256.Sum224(data)
		return sum[:]
	case SHA256:
		sum := sha256.Sum256(data)
		return sum[:]
	case SHA384:
		sum := sha512.Sum384(data)
		return sum[:]
	case SHA512:
		sum := sha512.Sum512(data)
		return sum[:]
	default:
		sum := sha256.Sum256(data)
		return sum[:]
	}
}

func (h Hash) Hash() crypto.Hash {
	switch h {
	case SHA1:
		return crypto.SHA1
	case SHA224:
		return crypto.SHA224
	case SHA256:
		return crypto.SHA256
	case SHA384:
		return crypto.SHA384
	case SHA512:
		return crypto.SHA512
	default:
		return crypto.SHA256
	}
}
