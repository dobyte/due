package xfile_test

import (
	"github.com/symsimmy/due/utils/xfile"
	"testing"
)

func TestWriteFile(t *testing.T) {
	err := xfile.WriteFile("./run/test.txt", []byte("hello world"))
	if err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
