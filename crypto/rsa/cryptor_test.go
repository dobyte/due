package rsa_test

import (
	"github.com/dobyte/due/crypto/rsa"
	"testing"
)

const pubPEM = `
-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAu8LqwgxXnKF7/1HPj12LpQzHV2UaVf+HELO6K0zutR6QYmtVq2uN
PPle5zpk4KHP5DLofVc650JhGO4jIlZ/SkCVyf9rZidS3iGlbFClo/+p1rH4ahCL
RBn5St/UvH3TJ8UH6a/+isP4wwQQNiVL56eixmtPt1zf7oSkHLd8Tu0o5aG6XdXs
vUH0x20WMtRzQgFDgpp/gyClRcYqGGm20bNTr2LfqVPVj0eVIIcukvACAHuFR63u
zNG9THr7fMwIYWnPnoA1kfFrkJS/7DL2AKklaiUsnHAGsPkzXKCcaeyJ1qVYi9k4
8+DFEWuVqKJdvG2vtjhh/zNBqb4S2sV0mQIDAQAB
-----END RSA PUBLIC KEY-----`

const priPEM = `
-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAu8LqwgxXnKF7/1HPj12LpQzHV2UaVf+HELO6K0zutR6QYmtV
q2uNPPle5zpk4KHP5DLofVc650JhGO4jIlZ/SkCVyf9rZidS3iGlbFClo/+p1rH4
ahCLRBn5St/UvH3TJ8UH6a/+isP4wwQQNiVL56eixmtPt1zf7oSkHLd8Tu0o5aG6
XdXsvUH0x20WMtRzQgFDgpp/gyClRcYqGGm20bNTr2LfqVPVj0eVIIcukvACAHuF
R63uzNG9THr7fMwIYWnPnoA1kfFrkJS/7DL2AKklaiUsnHAGsPkzXKCcaeyJ1qVY
i9k48+DFEWuVqKJdvG2vtjhh/zNBqb4S2sV0mQIDAQABAoIBAEkfYPPPgLNURIkr
oEzyHndZ4axMiJQjXsOHayJ/5JsO2yYpLQUEbs3nRCmDGVROUDtMBDUEKsFznYLr
Ay3VR99wBaXUXkw7Vk+CBP2I7ulOoSMmzlroNISCJQ8e2qfJzNk5J5q/2r7KEXBJ
fdLIdaYzJ/ZkRnhfqCoo6Azy/Gtzz3DsjOsJ2Z+bL59/D32Qn0oPCq9EEfTU8DQ1
rKXlexV/1RqFQoErl28sXKg4f81zxLHjbG8wJ3o95vdAn4VRnE+k7QSc9ShU7CBO
fWa1MXJ8irib1ERBt1N5P32vtFwjPT1XLGkzcGeKeSNkG+wqjzxwDohkgAARqJlj
wv2jYXECgYEA9AoCvdX0naxCDqMhE38E1zP/xcpaXK79uc6MB+T/7sNoyiy/aB6E
/ib3ZA81IdKupSP7HzTwBeSMTEmLQ5LjkyNDyr85Zj2YHXfXrkrEkZhFJ2LQwvn/
8S1yTODum+7f8DajSLW5z/qOTbUBbT5SF4eKwcObkn4SSVdF5S4V4v0CgYEAxPbI
ZxC5WiAG1mrsKZKzRAsNuwA/4hCZbQYgzH9R+uYhloQRt+Q3R46hau0FDtkIva+d
iHsHlbFsJ+PWQ51OaxmGpy9bkxr+28Ksa/XW0yTgrNztO8knij4z5Bi+6QWMwHS+
aS5wmSOUmS8gQKAXKyAKctpb+LuT+B79sjqwcM0CgYEA0b2A2bN3h8QzCe1+UglL
GcKhQ1dFDn9/piA1Ddvtc0ITYB/RaiVA3EaVPTQs0CMI4vnnrMyMtiPVyQM0ZCFs
4lreuvRa2tp5UGpdvniYNSIP3Wf6UHkZVilfIV/485/8a7Ip6CX3yx5nC7ZTwZZc
a8icoygBH4inIs2VTwGq4ekCgYEAjyDmMD8u8hcj4NyCERPRwThnGeTsh1KYq2kw
nGpJIJHrBn2igocMxKsZEaJ7cna0q2LajzsYH+d2OOaP5UKCocFC4GrBmPydBwVI
VounOHgr7HH+0tsyKHtbKf3xfVPTHGe5lqVwnVgFu+tK/KtZKrV14lBbVTy6Iiwj
H0kWvmECgYEAjL7p7Oxy2OEyTqx/nYNf/5VcN+HqB5kvkbcxBVA28mqa7ST5xhy7
mbLBPHHTu0YVRiQW2fl+yAvOm/ESZrgWaMt7Kx2ugaknx/Uj1lYBEu14oVQsAzMw
cUV1Uhl8Ew0ap4pCC1Bk/YcyswOlYFh9baQB+JNSHVkBJxPfeSgJDD0=
-----END RSA PRIVATE KEY-----`

func TestCryptor_Encrypt(t *testing.T) {
	c := rsa.NewCryptor(
		rsa.WithPublicKey(pubPEM),
		rsa.WithPrivateKey(priPEM),
	)

	plaintext, err := c.Encrypt([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	v, err := c.Decrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(v))
}
