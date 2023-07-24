package adapters

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
)

type FilesystemRepo struct {
	basePath string
}

func NewFileSystemRepository(basePath string) (*FilesystemRepo, error) {
	if basePath == "" {
		return nil, nil // no path, return empty repository
	}

	r := &FilesystemRepo{basePath: basePath}

	if err := os.MkdirAll(r.basePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", basePath, err)
	}

	return r, nil
}

func (r *FilesystemRepo) WriteFile(path string, contents io.Reader) error {
	filePath := filepath.Join(r.basePath, path)
	parentDirectory := filepath.Dir(filePath)

	if err := os.MkdirAll(parentDirectory, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", parentDirectory, err)
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			logrus.Errorf("failed to close file %s: %v", file.Name(), err)
		}
	}(file)

	_, err = io.Copy(file, contents)
	if err != nil {
		return fmt.Errorf("failed to write file contents: %w", err)
	}

	return nil

}
