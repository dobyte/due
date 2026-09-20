package log

import (
	"bufio"
	"os"
	"sync"
)

type StdLogger struct {
	mu     sync.Mutex
	file   *os.File
	writer *bufio.Writer
}

func NewStdLogger(filePath string) *StdLogger {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}

	l := &StdLogger{}
	l.file = file
	l.writer = bufio.NewWriter(l.file)

	return l
}

func (l *StdLogger) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	n, err := l.writer.Write(b)
	if err != nil {
		return 0, err
	}

	if err = l.writer.Flush(); err != nil {
		return 0, err
	}

	return n, nil
}

func (l *StdLogger) Close() error {
	return l.file.Close()
}
