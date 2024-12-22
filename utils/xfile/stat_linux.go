package xfile

import (
	"syscall"
	"time"
)

// CreateTime 获取文件创建时间
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Stat_t)

	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
