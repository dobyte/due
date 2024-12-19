package writer

import (
	"bufio"
	"fmt"
	"github.com/dobyte/due/v2/utils/xfile"
	gzip "github.com/klauspost/pgzip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	_ = 1 << (10 * iota)
	KB
	MB
	GB
	TB
)

type Writer struct {
	opts     *options
	fileDir  string
	fileName string
	fileExt  string
	gzipExt  string
	loc      *time.Location
	size     int64
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	tag      string
	version  int64
	acc      atomic.Int64
	chWrite  chan []byte
	flushing bool
}

func NewWriter(opts ...Option) *Writer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	w := &Writer{}
	w.opts = o
	w.init()

	return w
}

func (w *Writer) init() {
	if loc, err := time.LoadLocation(w.opts.timezone); err != nil {
		w.loc = time.Local
	} else {
		w.loc = loc
	}

	path, file := filepath.Split(w.opts.filePath)
	list := strings.Split(file, ".")
	switch c := len(list); c {
	case 1:
		w.fileName = list[0]
	default:
		w.fileName, w.fileExt = strings.Join(list[:c-1], "."), "."+list[c-1]
	}

	w.fileDir = path
	w.gzipExt = ".gz"
	w.chWrite = make(chan []byte, 4096)

	if err := w.sureFileMark(); err != nil {
		return
	}

	if err := w.tryRotateFile(); err != nil {
		return
	}
}

// 写入数据
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.mu.TryLock() {
		defer w.mu.Unlock()

		return w.flushToFile(p)
	} else {
		w.chWrite <- p
		w.acc.Add(1)

		w.tryFlushToFile()

		return len(p), nil
	}
}

// Close 关闭写入器
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	_, _ = w.flushToFile()

	close(w.chWrite)

	return w.file.Close()
}

// 尝试将数据刷入文件中
func (w *Writer) tryFlushToFile() {
HEAD:
	if w.flushing {
		return
	}

	if w.mu.TryLock() {
		_, _ = w.flushToFile()

		w.mu.Unlock()
	} else {
		goto HEAD
	}
}

// 写入将缓冲区数据写入文件
func (w *Writer) flushToFile(b ...[]byte) (n int, err error) {
	if w.file == nil {
		if err = w.openFile(); err != nil {
			return
		}
	}

	if acc := w.acc.Load(); acc > 0 {
		w.flushing = true

		defer func() {
			w.flushing = false
		}()

		for p := range w.chWrite {
			if _, err = w.writer.Write(p); err != nil {
				return
			}

			w.acc.Add(-1)

			acc--

			if acc == 0 {
				break
			}
		}
	}

	if len(b) > 0 {
		if n, err = w.writer.Write(b[0]); err != nil {
			return
		}

		_ = w.writer.Flush()
	}

	return
}

// 打开文件
func (w *Writer) openFile() error {
	if _, err := os.Stat(w.fileDir); err != nil {
		if err = os.MkdirAll(filepath.Dir(w.opts.filePath), 0644); err != nil {
			return err
		}
	}

	if tag := w.makeFileTag(w.now()); tag == w.tag {
		w.version++
	} else {
		w.tag = tag
		w.version = 1
	}

	file, err := os.OpenFile(w.opts.filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	w.size += fi.Size()
	w.file = file

	if w.writer == nil {
		w.writer = bufio.NewWriter(file)
	} else {
		w.writer.Reset(file)
	}

	return nil
}

// 尝试翻滚文件
func (w *Writer) tryRotateFile() error {
	fi, err := xfile.Stat(w.opts.filePath)
	if err != nil {
		return err
	}

	tag := w.makeFileTag(fi.CreateTime())

	if tag == w.tag {
		return nil
	}

	filePath := filepath.Join(w.fileDir, w.makeFileName(tag, w.version, w.fileExt))

	if err = os.Rename(w.opts.filePath, filePath); err != nil {
		return err
	}

	if !w.opts.compress {
		return nil
	}

	gzipPath := filepath.Join(w.fileDir, w.makeFileName(tag, w.version, w.gzipExt))

	return w.compressFile(gzipPath, filePath)
}

// 翻滚文件
func (w *Writer) rotateFile() error {
	if w.file == nil {
		return nil
	}

	if err := w.file.Close(); err != nil {
		return err
	}

	filePath := filepath.Join(w.fileDir, w.makeFileName(w.tag, w.version, w.fileExt))

	if err := os.Rename(w.opts.filePath, filePath); err != nil {
		return err
	}

	w.size = 0

	return w.openFile()
}

// 压缩文件
func (w *Writer) compressFile(dst, src string) (err error) {
	var (
		srcFile *os.File
		dstFile *os.File
	)

	if srcFile, err = os.Open(src); err != nil {
		return
	}

	defer func() {
		_ = srcFile.Close()

		if err == nil {
			_ = os.Remove(src)
		}
	}()

	if dstFile, err = os.Create(dst); err != nil {
		return err
	}

	defer func() {
		_ = dstFile.Close()
	}()

	dstWriter := gzip.NewWriter(dstFile)

	defer func() {
		_ = dstWriter.Close()
	}()

	if _, err = io.Copy(dstWriter, bufio.NewReader(srcFile)); err != nil {
		return
	}

	return
}

// 确定文件
func (w *Writer) sureFileMark() error {
	entries, err := os.ReadDir(w.fileDir)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		if len(fileName) < len(w.fileName)+len(w.fileExt)+1 {
			continue
		}

		if w.fileName != fileName[0:len(w.fileName)] {
			continue
		}

		var tags []string

		switch {
		case w.fileExt == fileName[len(fileName)-len(w.fileExt):]:
			tags = strings.Split(fileName[len(w.fileName):len(fileName)-len(w.fileExt)], ".")
		case w.gzipExt == fileName[len(fileName)-len(w.gzipExt):]:
			tags = strings.Split(fileName[len(w.fileName):len(fileName)-len(w.gzipExt)], ".")
		default:
			continue
		}

		fmt.Println("-----------------", tags)

		switch len(tags) {
		case 2:
			if version, err := strconv.ParseInt(tags[1], 10, 64); err != nil {
				continue
			} else {
				w.version = version
			}
		case 3:
			if version, err := strconv.ParseInt(tags[2], 10, 64); err != nil {
				continue
			} else {
				w.tag = tags[1]
				w.version = version
			}
		default:
			continue
		}

		break
	}

	return nil
}

// 生成文件名称
func (w *Writer) makeFileName(tag string, version int64, fileExt string) string {
	if tag == "" {
		return fmt.Sprintf("%s.%d%s", w.fileName, version, fileExt)
	} else {
		return fmt.Sprintf("%s.%s.%d%s", w.fileName, tag, version, fileExt)
	}
}

// 生成文件标签
func (w *Writer) makeFileTag(t time.Time) string {
	switch w.opts.fileRotate {
	case FileRotateByYear:
		return t.Format("2006")
	case FileRotateByMonth:
		return t.Format("200601")
	case FileRotateByDay:
		return t.Format("20060102")
	case FileRotateByHour:
		return t.Format("2006010215")
	case FileRotateByMinute:
		return t.Format("200601021504")
	case FileRotateBySecond:
		return t.Format("20060102150405")
	default:
		return ""
	}
}

// 获取当前时间
func (w *Writer) now() time.Time {
	return time.Now().In(w.loc)
}
