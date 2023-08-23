package xvalidate_test

import (
	"github.com/dobyte/due/v2/utils/xvalidate"
	"testing"
)

// "XXXX-XXXXXXX"
// "XXXX-XXXXXXXX"
// "XXX-XXXXXXX"
// "XXX-XXXXXXXX"
// "XXXXXXX"
// "XXXXXXXX"
func TestIsTelephone(t *testing.T) {
	t.Log(xvalidate.IsTelephone("0285-5554540"))
	t.Log(xvalidate.IsTelephone("0285-55545401"))
	t.Log(xvalidate.IsTelephone("028-5554540"))
	t.Log(xvalidate.IsTelephone("028-55545401"))
	t.Log(xvalidate.IsTelephone("5554540"))
	t.Log(xvalidate.IsTelephone("55545401"))
}

func TestIsEmail(t *testing.T) {
	t.Log(xvalidate.IsEmail("yuebanfuxiao@gmail.com"))
	t.Log(xvalidate.IsEmail("yuebanfuxiao"))
}

func TestIsUrl(t *testing.T) {
	t.Log(xvalidate.IsUrl("http://www.baidu.com"))
	t.Log(xvalidate.IsUrl("HTTP://WWW.BAIDU.COM"))
	t.Log(xvalidate.IsUrl("HTTP://a.b"))
	t.Log(xvalidate.IsUrl("HTTPs://a.b"))
}

func TestIsDigit(t *testing.T) {
	t.Log(xvalidate.IsDigit("11"))
	t.Log(xvalidate.IsDigit("11."))
	t.Log(xvalidate.IsDigit("11.1"))
	t.Log(xvalidate.IsDigit("011.1"))
	t.Log(xvalidate.IsDigit("aa.1"))
}
