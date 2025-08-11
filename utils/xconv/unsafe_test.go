package xconv_test

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/dobyte/due/v2/utils/xconv"
)

func TestStringToBytes(t *testing.T) {
	s := "abc"
	b := xconv.StringToBytes(s)

	fmt.Printf("string underlying array pointer: %p\n", unsafe.StringData(s))
	fmt.Printf("slice underlying array pointer: %p\n", unsafe.SliceData(b))
}

func TestBytesToString(t *testing.T) {
	b := []byte("abc")
	s := xconv.BytesToString(b)

	fmt.Printf("slice underlying array pointer: %p\n", unsafe.SliceData(b))
	fmt.Printf("string underlying array pointer: %p\n", unsafe.StringData(s))
}
