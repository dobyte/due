package stat

import (
	"os"
	"time"
)

type FileInfo interface {
	// Name returns the file name.
	Name() string
	// Size returns the file size.
	Size() int64
	// Mode returns the file mode.
	Mode() os.FileMode
	// IsDir reports whether the file is a directory.
	IsDir() bool
	// IsFile reports whether the file is a regular file.
	IsFile() bool
	// Sys returns the underlying system data.
	Sys() any
	// CreateTime returns the file creation time.
	CreateTime() time.Time
	// ModifyTime returns the file modification time.
	ModifyTime() time.Time
}

type fileStat struct {
	fi os.FileInfo
}

func Stat(filePath string) (FileInfo, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return &fileStat{fi: fi}, nil
}

// Name returns the file name.
func (fs *fileStat) Name() string {
	return fs.fi.Name()
}

// Size returns the file size.
func (fs *fileStat) Size() int64 {
	return fs.fi.Size()
}

// Mode returns the file mode.
func (fs *fileStat) Mode() os.FileMode {
	return fs.fi.Mode()
}

// ModifyTime returns the file modification time.
func (fs *fileStat) ModifyTime() time.Time {
	return fs.fi.ModTime()
}

// IsDir reports whether the file is a directory.
func (fs *fileStat) IsDir() bool {
	return fs.fi.IsDir()
}

// IsFile reports whether the file is a regular file.
func (fs *fileStat) IsFile() bool {
	return !fs.IsDir()
}

// Sys returns the underlying system data.
func (fs *fileStat) Sys() any {
	return fs.fi.Sys()
}
