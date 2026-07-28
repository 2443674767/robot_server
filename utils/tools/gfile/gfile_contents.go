package gfile

import (
	"fmt"
	"io"
	"os"
)

// GetContents returns the file content of `path` as string.
// It returns en empty string if it fails reading.
func GetContents(path string) string {
	return string(GetBytes(path))
}

// GetBytes returns the file content of `path` as []byte.
// It returns nil if it fails reading.
func GetBytes(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// putContents puts binary content to file of `path`.
func putContents(path string, data []byte, flag int, perm os.FileMode) error {
	// It supports creating file of `path` recursively.
	dir := Dir(path)
	if !Exists(dir) {
		if err := Mkdir(dir); err != nil {
			return err
		}
	}
	// Opening file with given `flag` and `perm`.
	f, err := OpenWithFlagPerm(path, flag, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	// Write data.
	var n int
	if n, err = f.Write(data); err != nil {
		return fmt.Errorf(`Write data to file "%s" failed，err：%s`, path, err.Error())
	} else if n < len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// PutBytes puts binary `content` to file of `path`.
// It creates file of `path` recursively if it does not exist.
func PutBytes(path string, content []byte) error {
	return putContents(path, content, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DefaultPermOpen)
}
