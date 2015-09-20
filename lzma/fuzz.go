// +build go-fuzz
package lzma

import (
	"bytes"
	"io"
	"io/ioutil"
)

func Fuzz(data []byte) int {
	r, err := NewReader(bytes.NewBuffer(data))
	if err != nil {
		if r != nil {
			panic("error but reader is not nil")
		}
		return 0
	}
	io.Copy(ioutil.Discard, r)
	return 0
}
