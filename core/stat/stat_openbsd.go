//go:build openbsd
// +build openbsd

package stat

import (
	"syscall"
	"time"
)

// CreateTime returns the file creation time.
func (fs *fileStat) CreateTime() time.Time {
	stat := fs.fi.Sys().(*syscall.Stat_t)

	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
}
