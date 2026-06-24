package common

import (
	"os"
	"path/filepath"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
)

// WriteDstFile writes a dst.File to disk using decorator.Fprint.
func WriteDstFile(file *dst.File, dstPath string) error {
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return decorator.Fprint(f, file)
}

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
