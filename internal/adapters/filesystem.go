package adapters

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type filesystemRepo struct {
	basePath string
}

func NewFileSystemRepository(basePath string) (*filesystemRepo, error) {
	r := &filesystemRepo{basePath: basePath}

	if err := os.MkdirAll(r.basePath, os.ModePerm); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *filesystemRepo) WriteFile(_ context.Context, path string, contents io.Reader) error {
	filePath := filepath.Join(r.basePath, path)

	err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, contents)
	if err != nil {
		return err
	}

	return nil

}
