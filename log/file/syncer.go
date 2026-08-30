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
	"github.com/dobyte/due/v2/utils/xos"
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
	fileTag     atomic.Pointer[string]
	fileVersion int64
	mu          sync.Mutex
	chMu        sync.Mutex
	size        int64
	file        *os.File
	writer      *bufio.Writer
	acc         atomic.Int64
	chEntry     chan *entry
	closing     atomic.Bool
	flushing    atomic.Bool
	wg          sync.WaitGroup
	formatter   internal.Formatter
	pool        sync.Pool
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

	if err := s.init(); err != nil {
		return nil
	}

	s.cleanExpiredFiles()

	go s.tickRotateFile()

	return s
}

func (s *Syncer) init() error {
	path, file := filepath.Split(s.opts.path)
	list := strings.Split(file, ".")
	switch c := len(list); c {
	case 1:
		s.fileName = list[0]
	default:
		s.fileName, s.fileExt = strings.Join(list[:c-1], "."), "."+list[c-1]
	}

	s.fileDir = path
	s.chEntry = make(chan *entry, 4096)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.pool = sync.Pool{New: func() any { return &entry{} }}

	if s.opts.format == FormatJson {
		s.formatter = internal.NewJsonFormatter()
	} else {
		s.formatter = internal.NewTextFormatter()
	}

	if err := s.parseFileMark(); err != nil {
		return err
	}

	fi, err := xos.Stat(s.opts.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}

	fileTag := s.makeFileTag(fi.CreateTime())

	if fileTag == s.getFileTag() {
		return nil
	}

	if err = s.doRotateFile(fileTag, s.fileVersion); err != nil {
		return err
	}

	return nil
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

	e := s.pool.Get().(*entry)
	e.now = entity.Now
	e.buf = s.formatter.Format(entity)

	return s.doWrite(e)
}

// 执行写入日志操作
func (s *Syncer) doWrite(e *entry) error {
	if s.mu.TryLock() {
		defer s.mu.Unlock()

		if s.closing.Load() {
			s.releaseEntry(e)
			return errors.ErrSyncerClosed
		}

		return s.flushToFile(e)
	} else {
		s.chMu.Lock()
		if s.closing.Load() {
			s.chMu.Unlock()
			s.releaseEntry(e)
			return errors.ErrSyncerClosed
		}

		s.acc.Add(1)
		s.chEntry <- e
		s.chMu.Unlock()

		s.tryFlushToFile()

		return nil
	}
}

// Close 关闭同步器
func (s *Syncer) Close() error {
	s.chMu.Lock()
	if !s.closing.CompareAndSwap(false, true) {
		s.chMu.Unlock()
		return errors.ErrSyncerClosed
	}
	s.chMu.Unlock()

	s.cancel()

	s.mu.Lock()
	e := s.pool.Get().(*entry)
	e.now = xtime.Now()
	s.flushToFile(e)
	s.wg.Wait()
	file := s.file
	s.file = nil
	s.mu.Unlock()

	if file != nil {
		_ = file.Sync()

		return file.Close()
	} else {
		return nil
	}
}

// 尝试将数据刷入文件中
func (s *Syncer) tryFlushToFile() {
	if s.closing.Load() {
		return
	}

	if s.flushing.Load() {
		return
	}

	s.mu.Lock()
	_ = s.flushToFile()
	s.mu.Unlock()
}

// 写入将缓冲区数据写入文件
func (s *Syncer) flushToFile(e ...*entry) error {
	if err := s.flushToWriter(len(e) > 0); err != nil {
		if len(e) > 0 {
			s.releaseEntry(e[0])
		}
		return err
	}

	if len(e) > 0 {
		return s.writeEntry(e[0], true)
	} else {
		return nil
	}
}

// 写入将缓冲区数据写入writer
func (s *Syncer) flushToWriter(isOpenFile bool) error {
	acc := s.acc.Load()

	if acc > 0 || isOpenFile {
		if s.file == nil {
			if err := s.openFile(); err != nil {
				return err
			}
		}
	}

	if acc > 0 {
		s.flushing.Store(true)
		defer s.flushing.Store(false)

	FLUSH_LOOP:
		for acc > 0 {
			select {
			case ent := <-s.chEntry:
				s.acc.Add(-1)

				if err := s.writeEntry(ent, false); err != nil {
					return err
				}

				acc--
			default:
				break FLUSH_LOOP
			}
		}
	}

	return nil
}

// 释放日志实体
func (s *Syncer) releaseEntry(e *entry) {
	if e.buf != nil {
		e.buf.Release()
		e.buf = nil
	}

	s.pool.Put(e)
}

// 写入日志
func (s *Syncer) writeEntry(e *entry, isAutoFlush bool) error {
	defer s.releaseEntry(e)

	if s.opts.rotate != RotateNone {
		if fileTag := s.makeFileTag(e.now); fileTag != s.getFileTag() {
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

	if s.opts.maxSize > 0 && s.size >= s.opts.maxSize {
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

			if s.makeFileTag(now) != s.getFileTag() {
				e := s.pool.Get().(*entry)
				e.now = now
				s.doWrite(e)
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

	if err := s.file.Sync(); err != nil {
		return err
	}

	if err := s.file.Close(); err != nil {
		return err
	}

	return s.doRotateFile(s.getFileTag(), s.fileVersion)
}

// 处理翻转文件
func (s *Syncer) doRotateFile(fileTag string, fileVersion int64) (err error) {
	filePath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, s.fileExt))
	gzipPath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, gzipExt))

	for {
		if _, statErr := os.Stat(filePath); statErr == nil {
			// 文件已存在，尝试下一个版本号
		} else if os.IsNotExist(statErr) {
			if _, gzErr := os.Stat(gzipPath); gzErr == nil {
				// gzip 文件已存在，尝试下一个版本号
			} else if os.IsNotExist(gzErr) {
				break // 两者都不存在，找到可用版本号
			} else {
				return gzErr
			}
		} else {
			return statErr
		}

		fileVersion++
		filePath = filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, s.fileExt))
		gzipPath = filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, gzipExt))
	}

	if err = os.Rename(s.opts.path, filePath); err != nil {
		return
	}

	if err = s.openFile(); err != nil {
		return
	}

	if !s.opts.compress {
		s.cleanExpiredFiles()
		return
	}

	s.wg.Go(func() {
		_ = s.compressFile(gzipPath, filePath)
		s.cleanExpiredFiles()
	})

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
		if closeErr := dstWriter.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err = io.Copy(dstWriter, srcFile); err != nil {
		return
	}

	return
}

// 清理过期文件
func (s *Syncer) cleanExpiredFiles() {
	if s.opts.maxAge <= 0 {
		return
	}

	entries, err := os.ReadDir(s.fileDir)
	if err != nil {
		return
	}

	now := xtime.Now()
	currentBase := filepath.Base(s.opts.path)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()

		if fileName == currentBase {
			continue
		}

		if len(fileName) < len(s.fileName)+len(s.fileExt)+1 {
			continue
		}

		if s.fileName != fileName[0:len(s.fileName)] {
			continue
		}

		var (
			fileTime time.Time
			fileTags []string
			matched  bool
		)

		switch {
		case s.fileExt == fileName[len(fileName)-len(s.fileExt):]:
			fileTags = strings.Split(fileName[len(s.fileName):len(fileName)-len(s.fileExt)], ".")
			matched = true
		case gzipExt == fileName[len(fileName)-len(gzipExt):]:
			fileTags = strings.Split(fileName[len(s.fileName):len(fileName)-len(gzipExt)], ".")
			matched = true
		}

		if !matched {
			continue
		}

		switch len(fileTags) {
		case 2:
			fileTime = s.fileModTime(entry)
		case 3:
			if t, ok := s.parseFileTagTime(fileTags[1]); ok {
				fileTime = t
			} else {
				fileTime = s.fileModTime(entry)
			}
		default:
			fileTime = s.fileModTime(entry)
		}

		if now.Sub(fileTime) > s.opts.maxAge {
			_ = os.Remove(filepath.Join(s.fileDir, fileName))
		}
	}
}

// 从 fileTag 字符串解析时间
func (s *Syncer) parseFileTagTime(tag string) (time.Time, bool) {
	switch s.opts.rotate {
	case RotateYear:
		if t, err := xtime.Parse("2006", tag); err == nil {
			return t, true
		}
	case RotateMonth:
		if t, err := xtime.Parse("200601", tag); err == nil {
			return t, true
		}
	case RotateWeek:
		if len(tag) == 6 {
			year, err1 := strconv.Atoi(tag[:4])
			week, err2 := strconv.Atoi(tag[4:])
			if err1 == nil && err2 == nil && week >= 1 && week <= 53 {
				// 1月4日始终属于 ISO 第1周，据此推算出第1周的周一
				jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, xtime.GetLocation())
				weekday := int(jan4.Weekday())
				if weekday == 0 {
					weekday = 7 // 周日
				}
				monday := jan4.AddDate(0, 0, -(weekday - 1))
				return monday.AddDate(0, 0, (week-1)*7), true
			}
		}
	case RotateDay:
		if t, err := xtime.Parse("20060102", tag); err == nil {
			return t, true
		}
	case RotateHour:
		if t, err := xtime.Parse("2006010215", tag); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// 获取文件修改时间（兜底）
func (s *Syncer) fileModTime(entry os.DirEntry) time.Time {
	info, err := entry.Info()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// 打开文件
func (s *Syncer) openFile() error {
	if _, err := os.Stat(s.fileDir); err != nil {
		if err = os.MkdirAll(s.fileDir, 0755); err != nil {
			return err
		}
	}

	if fileTag := s.makeFileTag(xtime.Now()); fileTag == s.getFileTag() {
		s.fileVersion++
	} else {
		s.setFileTag(fileTag)
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
		s.writer = bufio.NewWriterSize(file, s.opts.bufferSize)
	} else {
		s.writer.Reset(file)
	}

	return nil
}

// 解析文件标识
func (s *Syncer) parseFileMark() error {
	entries, err := os.ReadDir(s.fileDir)
	if err != nil {
		if os.IsNotExist(err) {
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
		case gzipExt == fileName[len(fileName)-len(gzipExt):]:
			fileTags = strings.Split(fileName[len(s.fileName):len(fileName)-len(gzipExt)], ".")
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

	if fileTag := s.makeFileTag(xtime.Now()); fileTag != s.getFileTag() {
		s.setFileTag(fileTag)
		s.fileVersion = 0
	}

	return nil
}

// 过滤文件标识
func (s *Syncer) filterFileMark(fileTag string, fileVersion int64) {
	switch {
	case fileTag > s.getFileTag():
		s.setFileTag(fileTag)
		s.fileVersion = fileVersion
	case fileTag == s.getFileTag():
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

// getFileTag 获取当前文件标签
func (s *Syncer) getFileTag() string {
	if tag := s.fileTag.Load(); tag != nil {
		return *tag
	}
	return ""
}

// setFileTag 设置当前文件标签
func (s *Syncer) setFileTag(fileTag string) {
	s.fileTag.Store(&fileTag)
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
