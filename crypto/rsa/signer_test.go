/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/11/1 1:42 上午
 * @Desc: TODO
 */

package rsa_test

import (
	"testing"

	"github.com/dobyte/due/crypto/rsa"
)

func TestSigner_Sign(t *testing.T) {
	digest := []byte("h")

	signer := rsa.NewSigner(
		rsa.WithSignerHash(rsa.SHA256),
		rsa.WithSignerPadding(rsa.PKCS),
		rsa.WithSignerPrivateKey("./pem/key.pem"),
	)

	signature, err := signer.Sign(digest)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(signature)
}
