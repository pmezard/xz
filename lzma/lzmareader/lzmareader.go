package main

import (
	"fmt"
	"github.com/ulikunitz/xz/lzma"
	"io"
	"io/ioutil"
	"os"
)

func Decode(path string) error {
	fp, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fp.Close()
	r, err := lzma.NewReader(fp)
	if err != nil {
		return err
	}
	_, err = io.Copy(ioutil.Discard, r)
	return err
}

func main() {
	path := os.Args[1]
	err := Decode(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}
