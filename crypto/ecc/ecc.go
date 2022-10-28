package ecc

import (
	"encoding/pem"
	"github.com/dobyte/due/utils/xconv"
	"github.com/dobyte/due/utils/xpath"
	"io/ioutil"
)

const Name = "ecc"

func loadKey(key string) (*pem.Block, error) {
	isFile, err := xpath.IsFile(key)
	if err != nil {
		return nil, err
	}

	var buffer []byte

	if isFile {
		buffer, err = ioutil.ReadFile(key)
		if err != nil {
			return nil, err
		}
	} else {
		buffer = xconv.StringToBytes(key)
	}

	block, _ := pem.Decode(buffer)

	return block, nil
}
