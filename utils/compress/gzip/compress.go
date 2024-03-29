package gzip

import (
	"bytes"
	"compress/gzip"
	"sync"
)

var (
	spWriter sync.Pool
	spBuffer sync.Pool
)

func init() {
	// 公共对象池,更极致的优化可以建多个池
	spWriter = sync.Pool{New: func() interface{} {
		buf := new(bytes.Buffer)
		return gzip.NewWriter(buf)
	}}
	spBuffer = sync.Pool{New: func() interface{} {
		return new(bytes.Buffer)
	}}
}

func Encode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
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

//func Encode(input []byte) ([]byte, error) {
//	buf := spBuffer.Get().(*bytes.Buffer)
//	gzipWriter := spWriter.Get().(*gzip.Writer)
//	gzipWriter.Reset(buf)
//	defer func() {
//		// 归还buff
//		buf.Reset()
//		spBuffer.Put(buf)
//		// 归还Writer
//		spWriter.Put(gzipWriter)
//	}()
//	// 创建一个新的 klugzip 输出流
//	_, err := gzipWriter.Write(input)
//
//	if err != nil {
//		_ = gzipWriter.Close()
//		return nil, err
//	}
//
//	if err := gzipWriter.Close(); err != nil {
//		return nil, err
//	}
//	// 返回压缩后的 bytes 数组
//	return buf.Bytes(), nil
//}
