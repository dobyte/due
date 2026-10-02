//go:build darwin
// +build darwin

package stat

import (
	"syscall"
	"time"
)

// CreateTime returns the file creation time.
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Stat_t)

	return time.Unix(stat.Ctimespec.Sec, stat.Ctimespec.Nsec)
}
