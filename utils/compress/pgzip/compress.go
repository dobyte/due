package pgzip

import (
	"bytes"
)
import "github.com/klauspost/pgzip"

func Encode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
	// 创建一个新的 klugzip 输出流
	gzipWriter := pgzip.NewWriter(&buf)
	gzipWriter.SetConcurrency(1024, 1)
	// 将 input byte 数组写入到此输出流中
	_, err := gzipWriter.Write(input)
	if err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	// 返回压缩后的 bytes 数组
	return buf.Bytes(), nil
}

// Decode gzip解压二进制数组
func Decode(input []byte) ([]byte, error) {
	// 创建一个新的 klugzip.Reader
	bytesReader := bytes.NewReader(input)
	gzipReader, err := pgzip.NewReader(bytesReader)
	if err != nil {
		return nil, err
	}
	defer func() {
		// defer 中关闭 gzipReader
		_ = gzipReader.Close()
	}()
	buf := new(bytes.Buffer)
	// 从 Reader 中读取出数据
	if _, err := buf.ReadFrom(gzipReader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
