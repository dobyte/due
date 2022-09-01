/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/31 3:37 下午
 * @Desc: TODO
 */

package utils

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func CheckIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		_, err := unix.IoctlGetTermios(int(v.Fd()), unix.TIOCGETA)
		return err == nil
	default:
		return false
	}
}
