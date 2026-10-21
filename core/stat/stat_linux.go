//go:build linux
// +build linux

package stat

import (
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// CreateTime returns the file creation time. The creation time (btime) is only
// exposed by statx, and falls back to the inode change time when the kernel or
// the file system does not support it.
func (fs *fileStat) CreateTime() time.Time {
	if createTime, ok := fs.statxCreateTime(); ok {
		return createTime
	}

	stat := fs.fi.Sys().(*syscall.Stat_t)

	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}

// statxCreateTime returns the file creation time reported by statx.
func (fs *fileStat) statxCreateTime() (time.Time, bool) {
	var stx unix.Statx_t

	if err := unix.Statx(unix.AT_FDCWD, fs.path, 0, unix.STATX_BTIME, &stx); err != nil {
		return time.Time{}, false
	}

	if stx.Mask&unix.STATX_BTIME == 0 || stx.Btime.Sec == 0 {
		return time.Time{}, false
	}

	return time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec)), true
}
