package xfile

import (
	"syscall"
	"time"
)

// CreateTime 获取文件创建时间
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Win32FileAttributeData)

	nsec := stat.CreationTime.Nanoseconds()

	return time.Unix(nsec/int64(time.Second), nsec%int64(time.Second))
}
