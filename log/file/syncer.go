package file

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log/internal"
	"github.com/dobyte/due/v2/utils/xfile"
	"github.com/dobyte/due/v2/utils/xtime"
)

const Name = "file"

const gzipExt = ".gz"

type Syncer struct {
	opts        *options
	ctx         context.Context
	cancel      context.CancelFunc
	fileDir     string
	fileName    string
	fileExt     string
	fileTag     string
	fileVersion int64
	gzipExt     string
	mu          sync.Mutex
	size        int64
	file        *os.File
	writer      *bufio.Writer
	acc         atomic.Int64
	chEntry     chan entry
	closing     atomic.Bool
	flushing    bool
	wg          sync.WaitGroup
	formatter   internal.Formatter
}

type entry struct {
	now time.Time
	buf internal.Buffer
}

func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Syncer{}
	s.opts = o
	s.init()

	return s
}

func (s *Syncer) init() {
	path, file := filepath.Split(s.opts.path)
	list := strings.Split(file, ".")
	switch c := len(list); c {
	case 1:
		s.fileName = list[0]
	default:
		s.fileName, s.fileExt = strings.Join(list[:c-1], "."), "."+list[c-1]
	}

	s.fileDir = path
	s.gzipExt = gzipExt
	s.chEntry = make(chan entry, 4096)
	s.ctx, s.cancel = context.WithCancel(context.Background())

	if s.opts.format == FormatJson {
		s.formatter = internal.NewJsonFormatter()
	} else {
		s.formatter = internal.NewTextFormatter()
	}

	defer func() {
		go s.tickRotateFile()
	}()

	if err := s.parseFileMark(); err != nil {
		return
	}

	fi, err := xfile.Stat(s.opts.path)
	if err != nil {
		return
	}

	fileTag := s.makeFileTag(fi.CreateTime())

	if fileTag == s.fileTag {
		return
	}

	if err = s.doRotateFile(fileTag, s.fileVersion); err != nil {
		return
	}
}

// Name 同步器名称
func (s *Syncer) Name() string {
	return Name
}

// Write 写入日志
func (s *Syncer) Write(entity *internal.Entity) error {
	if s.closing.Load() {
		return errors.ErrSyncerClosed
	}

	return s.doWrite(entry{
		buf: s.formatter.Format(entity),
		now: entity.Now,
	})
}

// 执行写入日志操作
func (s *Syncer) doWrite(e entry) error {
	if s.mu.TryLock() {
		defer s.mu.Unlock()

		if s.closing.Load() {
			return errors.ErrSyncerClosed
		}

		return s.flushToFile(e)
	} else {
		s.chEntry <- e
		s.acc.Add(1)

		s.tryFlushToFile()

		return nil
	}
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.closing.CompareAndSwap(false, true) {
		return errors.ErrSyncerClosed
	}

	s.cancel()

	_ = s.flushToFile()

	s.wg.Wait()

	if s.file == nil {
		return nil
	}

	return s.file.Close()
}

// 尝试将数据刷入文件中
func (s *Syncer) tryFlushToFile() {
HEAD:
	if s.flushing {
		return
	}

	if s.mu.TryLock() {
		_ = s.flushToFile()

		s.mu.Unlock()
	} else {
		goto HEAD
	}
}

// 写入将缓冲区数据写入文件
func (s *Syncer) flushToFile(e ...entry) error {
	acc := s.acc.Load()

	if acc > 0 || len(e) > 0 {
		if s.file == nil {
			if err := s.openFile(); err != nil {
				return err
			}
		}
	}

	if acc > 0 {
		s.flushing = true

		defer func() {
			s.flushing = false
		}()

		for e := range s.chEntry {
			s.acc.Add(-1)

			if err := s.writeEntry(e, false); err != nil {
				return err
			}

			acc--

			if acc == 0 {
				break
			}
		}
	}

	if len(e) > 0 {
		return s.writeEntry(e[0], true)
	} else {
		return nil
	}
}

// 写入日志
func (s *Syncer) writeEntry(e entry, isAutoFlush bool) error {
	if s.opts.rotate != RotateNone {
		if fileTag := s.makeFileTag(e.now); fileTag != s.fileTag {
			if err := s.writer.Flush(); err != nil {
				return err
			}

			if err := s.rotateFile(); err != nil {
				return err
			}
		}
	}

	if e.buf != nil {
		size, err := s.writer.Write(e.buf.Bytes())
		e.buf.Release()

		if err != nil {
			return err
		}

		s.size += int64(size)
	}

	if isAutoFlush {
		if err := s.writer.Flush(); err != nil {
			return err
		}
	}

	if s.size >= s.opts.maxSize {
		if err := s.writer.Flush(); err != nil {
			return err
		}

		if err := s.rotateFile(); err != nil {
			return err
		}
	}

	return nil
}

// 定时翻滚文件
func (s *Syncer) tickRotateFile() {
	if s.opts.rotate == RotateNone {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case now, ok := <-ticker.C:
			if !ok {
				return
			}

			if s.makeFileTag(now) != s.fileTag {
				s.doWrite(entry{now: now})
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// 翻滚文件
func (s *Syncer) rotateFile() error {
	if s.file == nil {
		return nil
	}

	if err := s.file.Close(); err != nil {
		return err
	}

	return s.doRotateFile(s.fileTag, s.fileVersion)
}

// 处理翻转文件
func (s *Syncer) doRotateFile(fileTag string, fileVersion int64) (err error) {
	filePath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, s.fileExt))

	if err = os.Rename(s.opts.path, filePath); err != nil {
		return
	}

	if err = s.openFile(); err != nil {
		return
	}

	if !s.opts.compress {
		return
	}

	gzipPath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, gzipExt))

	s.wg.Add(1)

	go func() {
		_ = s.compressFile(gzipPath, filePath)

		s.wg.Done()
	}()

	return
}

// 压缩文件
func (s *Syncer) compressFile(dst, src string) (err error) {
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

// 打开文件
func (s *Syncer) openFile() error {
	if _, err := os.Stat(s.fileDir); err != nil {
		if err = os.MkdirAll(filepath.Dir(s.opts.path), 0755); err != nil {
			return err
		}
	}

	if fileTag := s.makeFileTag(xtime.Now()); fileTag == s.fileTag {
		s.fileVersion++
	} else {
		s.fileTag = fileTag
		s.fileVersion = 1
	}

	file, err := os.OpenFile(s.opts.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	s.size = fi.Size()
	s.file = file

	if s.writer == nil {
		s.writer = bufio.NewWriter(file)
	} else {
		s.writer.Reset(file)
	}

	return nil
}

// 解析文件标识
func (s *Syncer) parseFileMark() error {
	entries, err := os.ReadDir(s.fileDir)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		if len(fileName) < len(s.fileName)+len(s.fileExt)+1 {
			continue
		}

		if s.fileName != fileName[0:len(s.fileName)] {
			continue
		}

		var fileTags []string

		switch {
		case s.fileExt == fileName[len(fileName)-len(s.fileExt):]:
			fileTags = strings.Split(fileName[len(s.fileName):len(fileName)-len(s.fileExt)], ".")
		case s.gzipExt == fileName[len(fileName)-len(s.gzipExt):]:
			fileTags = strings.Split(fileName[len(s.fileName):len(fileName)-len(s.gzipExt)], ".")
		default:
			continue
		}

		switch len(fileTags) {
		case 2:
			if fileVersion, err := strconv.ParseInt(fileTags[1], 10, 64); err != nil {
				continue
			} else {
				s.filterFileMark("", fileVersion)
			}
		case 3:
			if fileVersion, err := strconv.ParseInt(fileTags[2], 10, 64); err != nil {
				continue
			} else {
				s.filterFileMark(fileTags[1], fileVersion)
			}
		default:
			// ignore
		}
	}

	if fileTag := s.makeFileTag(xtime.Now()); fileTag != s.fileTag {
		s.fileTag = fileTag
		s.fileVersion = 0
	}

	return nil
}

// 过滤文件标识
func (s *Syncer) filterFileMark(fileTag string, fileVersion int64) {
	switch {
	case fileTag > s.fileTag:
		s.fileTag = fileTag
		s.fileVersion = fileVersion
	case fileTag == s.fileTag:
		if fileVersion > s.fileVersion {
			s.fileVersion = fileVersion
		}
	default:
		// ignore
	}
}

// 生成文件名称
func (s *Syncer) makeFileName(fileTag string, fileVersion int64, fileExt string) string {
	if fileTag == "" {
		return fmt.Sprintf("%s.%d%s", s.fileName, fileVersion, fileExt)
	} else {
		return fmt.Sprintf("%s.%s.%d%s", s.fileName, fileTag, fileVersion, fileExt)
	}
}

// 生成文件标签
func (s *Syncer) makeFileTag(t time.Time) string {
	switch s.opts.rotate {
	case RotateYear:
		return t.Format("2006")
	case RotateMonth:
		return t.Format("200601")
	case RotateWeek:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d%02d", year, week)
	case RotateDay:
		return t.Format("20060102")
	case RotateHour:
		return t.Format("2006010215")
	default:
		return ""
	}
}
