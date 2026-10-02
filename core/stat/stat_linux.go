//go:build linux
// +build linux

package stat

import (
	"syscall"
	"time"
)

// CreateTime returns the file creation time.
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Stat_t)

	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}
