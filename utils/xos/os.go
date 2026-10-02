package xos

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dobyte/due/v2/core/stat"
)

// Stat returns the file information of filePath.
func Stat(filePath string) (stat.FileInfo, error) {
	return stat.Stat(filePath)
}

// IsDir reports whether path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile reports whether path is a file.
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Split splits path into its directory, file name, name without extension and extension.
func Split(path string) (dir, file, name, ext string) {
	dir, file = filepath.Split(path)

	ext = filepath.Ext(file)
	if ext == file {
		// A hidden file that starts with a dot, such as .gitignore, is treated as having no
		// extension.
		name = file
		ext = ""
		return
	}

	if ext != "" {
		name = file[:len(file)-len(ext)]
		ext = ext[1:] // Strip the leading dot of the extension.
	} else {
		name = file
	}
	return
}

// WriteFile writes data to file, creating the parent directory when it does not exist.
func WriteFile(file string, data []byte) error {
	if path := filepath.Dir(file); !IsDir(path) {
		if err := os.MkdirAll(path, fs.ModePerm); err != nil {
			return err
		}
	}

	return os.WriteFile(file, data, fs.ModePerm)
}
