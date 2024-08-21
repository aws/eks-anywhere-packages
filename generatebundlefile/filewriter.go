package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultTmpFolder        = "generated"
	objectSeparator  string = "---\n"
)

func defaultFileOptions() *FileOptions {
	return &FileOptions{os.ModePerm}
}

func PersistentFile(op *FileOptions) {
	op.Permissions = os.ModePerm
}

type writer struct {
	dir string
}

type FileWriter interface {
	Write(fileName string, content []byte, f ...FileOptionsFunc) (path string, err error)
	WithDir(dir string) (FileWriter, error)
	CleanUp()
	Dir() string
}

type FileOptions struct {
	Permissions os.FileMode
}

type FileOptionsFunc func(op *FileOptions)

func NewWriter(dir string) (FileWriter, error) {
	newFolder := filepath.Join(dir, DefaultTmpFolder)
	if _, err := os.Stat(newFolder); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(newFolder, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("error creating directory [%s]: %v", dir, err)
		}
	}
	return &writer{dir: dir}, nil
}

func (t *writer) Write(fileName string, content []byte, f ...FileOptionsFunc) (string, error) {
	op := defaultFileOptions() // Default file options. -->> temporary file with default permissions
	for _, optionFunc := range f {
		optionFunc(op)
	}
	currentDir := t.dir
	filePath := filepath.Join(currentDir, fileName)
	err := os.WriteFile(filePath, content, op.Permissions)
	if err != nil {
		return "", fmt.Errorf("error writing to file [%s]: %v", filePath, err)
	}

	return filePath, nil
}

func (w *writer) WithDir(dir string) (FileWriter, error) {
	return NewWriter(filepath.Join(w.dir, dir))
}

func (t *writer) Dir() string {
	return t.dir
}

func (t *writer) CleanUp() {
	_, err := os.Stat(t.dir)
	if err == nil {
		_ = os.RemoveAll(t.dir)
	}
}

func ConcatYamlResources(resources ...[]byte) []byte {
	separator := []byte(objectSeparator)
	size := 0
	for _, resource := range resources {
		size += len(resource) + len(separator)
	}

	b := make([]byte, 0, size)
	b = append(b, separator...)
	for _, resource := range resources {
		b = append(b, resource...)
	}
	return b
}
