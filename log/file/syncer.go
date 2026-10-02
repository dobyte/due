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

// Name is the syncer name.
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

	go s.tickFlushFile()
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

// Name returns the syncer name.
func (s *Syncer) Name() string {
	return Name
}

// Write writes the given entity. It reports [errors.ErrSyncerClosed] when the syncer has been closed.
func (s *Syncer) Write(entity *internal.Entity) error {
	if s.closing.Load() {
		return errors.ErrSyncerClosed
	}

	e := s.pool.Get().(*entry)
	e.now = entity.Now
	e.buf = s.formatter.Format(entity)

	return s.doWrite(e)
}

// doWrite writes the entry to the file directly or enqueues it for a batched flush.
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

// Close closes the syncer, flushing and closing the underlying file.
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
	if s.writer != nil {
		_ = s.writer.Flush()
	}
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

// tryFlushToFile flushes buffered data to the file.
func (s *Syncer) tryFlushToFile() {
	if s.closing.Load() {
		return
	}

	s.mu.Lock()
	_ = s.flushToFile()
	s.mu.Unlock()
}

// flushToFile writes buffered data to the file and optionally writes the given entry.
func (s *Syncer) flushToFile(e ...*entry) error {
	if err := s.flushToWriter(len(e) > 0); err != nil {
		if len(e) > 0 {
			s.releaseEntry(e[0])
		}
		return err
	}

	if len(e) > 0 {
		return s.writeEntry(e[0], s.opts.flushInterval <= 0)
	} else {
		return nil
	}
}

// flushToWriter writes buffered data to the writer.
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

// releaseEntry releases the entry back to the pool.
func (s *Syncer) releaseEntry(e *entry) {
	if e.buf != nil {
		e.buf.Release()
		e.buf = nil
	}

	s.pool.Put(e)
}

// writeEntry writes the entry to the writer.
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

// tickFlushFile flushes the file periodically.
func (s *Syncer) tickFlushFile() {
	if s.opts.flushInterval <= 0 {
		return
	}

	ticker := time.NewTicker(s.opts.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.writer != nil {
				_ = s.writer.Flush()
			}
			s.mu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// tickRotateFile rotates the file periodically.
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

// rotateFile rotates the current file.
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

// doRotateFile rotates the current file to the given tag and version.
func (s *Syncer) doRotateFile(fileTag string, fileVersion int64) (err error) {
	filePath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, s.fileExt))
	gzipPath := filepath.Join(s.fileDir, s.makeFileName(fileTag, fileVersion, gzipExt))

	for {
		if _, statErr := os.Stat(filePath); statErr == nil {
			// The file already exists; try the next version number.
		} else if os.IsNotExist(statErr) {
			if _, gzErr := os.Stat(gzipPath); gzErr == nil {
				// The gzip file already exists; try the next version number.
			} else if os.IsNotExist(gzErr) {
				break // Neither exists; a usable version number has been found.
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

// compressFile compresses src into dst.
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

// cleanExpiredFiles removes files older than maxAge.
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

// parseFileTagTime parses a file tag into a time.
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
				// January 4 always belongs to ISO week 1, so week 1's Monday can be derived from it.
				jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, xtime.GetLocation())
				weekday := int(jan4.Weekday())
				if weekday == 0 {
					weekday = 7 // Sunday
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

// fileModTime returns the file modification time as a fallback.
func (s *Syncer) fileModTime(entry os.DirEntry) time.Time {
	info, err := entry.Info()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// openFile opens the log file.
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

// parseFileMark parses the file mark from existing files.
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

// filterFileMark updates the file tag and version from the given mark.
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

// makeFileName builds the file name for the tag, version and extension.
func (s *Syncer) makeFileName(fileTag string, fileVersion int64, fileExt string) string {
	if fileTag == "" {
		return fmt.Sprintf("%s.%d%s", s.fileName, fileVersion, fileExt)
	} else {
		return fmt.Sprintf("%s.%s.%d%s", s.fileName, fileTag, fileVersion, fileExt)
	}
}

// getFileTag returns the current file tag.
func (s *Syncer) getFileTag() string {
	if tag := s.fileTag.Load(); tag != nil {
		return *tag
	}
	return ""
}

// setFileTag sets the current file tag.
func (s *Syncer) setFileTag(fileTag string) {
	s.fileTag.Store(&fileTag)
}

// makeFileTag builds the file tag for the given time according to the rotation rule.
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
