package klugzip

import (
	"bytes"
	"github.com/klauspost/compress/gzip"
)

func Encode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
	// 创建一个新的 klugzip 输出流
	gzipWriter := gzip.NewWriter(&buf)
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

func Decode(input []byte) ([]byte, error) {
	// 创建一个新的 klugzip.Reader
	bytesReader := bytes.NewReader(input)
	gzipReader, err := gzip.NewReader(bytesReader)
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
