//go:build windows
// +build windows

package stat

import (
	"syscall"
	"time"
)

// CreateTime returns the file creation time.
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Win32FileAttributeData)

	nsec := stat.CreationTime.Nanoseconds()

	return time.Unix(nsec/int64(time.Second), nsec%int64(time.Second))
}
