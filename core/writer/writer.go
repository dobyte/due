package writer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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
	loc      *time.Location
	size     int64
	no       int64
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
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
	path, file := filepath.Split(w.opts.filePath)

	list := strings.Split(file, ".")
	switch c := len(list); c {
	case 1:
		w.fileName = list[0]
	default:
		w.fileName, w.fileExt = strings.Join(list[:c-1], "."), "."+list[c-1]
	}

	w.fileDir = path
	w.chWrite = make(chan []byte, 4096)

	if loc, err := time.LoadLocation(w.opts.timezone); err != nil {
		w.loc = time.Local
	} else {
		w.loc = loc
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
		if err = os.MkdirAll(filepath.Dir(w.opts.filePath), 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(w.opts.filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	w.size += fi.Size()
	w.file = f

	if w.writer == nil {
		w.writer = bufio.NewWriter(f)
	} else {
		w.writer.Reset(f)
	}

	return nil
}

// 翻滚文件
func (w *Writer) rotateFile(t time.Time, id int) error {
	if err := w.file.Close(); err != nil {
		return err
	}

	var newFileName string

	switch w.opts.fileRotate {
	case FileRotateByYear:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("2006"), id, w.fileExt)
	case FileRotateByMonth:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("200601"), id, w.fileExt)
	case FileRotateByDay:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("20060102"), id, w.fileExt)
	case FileRotateByHour:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("2006010215"), id, w.fileExt)
	case FileRotateByMinute:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("200601021504"), id, w.fileExt)
	case FileRotateBySecond:
		newFileName = fmt.Sprintf("%s.%s.%d%s", w.fileName, t.Format("20060102150405"), id, w.fileExt)
	default:
		newFileName = fmt.Sprintf("%s.%d%s", w.fileName, id, w.fileExt)
	}

	if err := os.Rename(w.opts.filePath, filepath.Join(w.fileDir, newFileName)); err != nil {
		return err
	}

	w.size = 0

	return w.openFile()
}

// 压缩文件
func (w *Writer) compressFile() {

}

// 获取当前时间
func (w *Writer) now() time.Time {
	return time.Now().In(w.loc)
}
