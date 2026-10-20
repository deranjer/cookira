// Package storage provides a swappable interface for attachment file storage.
// v1 implementation is local disk; a future S3-backed implementation can
// satisfy the same interface without touching handler code.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store interface {
	Save(ctx context.Context, key string, r io.Reader) (size int64, err error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Disk is a Store backed by a directory on the local filesystem. Uploads and
// downloads are streamed via io.Copy so the process never buffers a full
// file in memory, regardless of file size.
type Disk struct {
	root string
}

func NewDisk(root string) (*Disk, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creating storage root %q: %w", root, err)
	}
	return &Disk{root: root}, nil
}

func (d *Disk) path(key string) (string, error) {
	p := filepath.Join(d.root, filepath.Clean("/"+key))
	if !filepathHasPrefix(p, d.root) {
		return "", fmt.Errorf("invalid storage key %q", key)
	}
	return p, nil
}

func filepathHasPrefix(p, prefix string) bool {
	rel, err := filepath.Rel(prefix, p)
	if err != nil {
		return false
	}
	return rel != ".." && !hasDotDotPrefix(rel)
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[0] == '.' && rel[1] == '.'
}

func (d *Disk) Save(ctx context.Context, key string, r io.Reader) (int64, error) {
	p, err := d.path(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(p)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, fmt.Errorf("writing %q: %w", key, err)
	}
	return n, nil
}

func (d *Disk) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (d *Disk) Delete(ctx context.Context, key string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
