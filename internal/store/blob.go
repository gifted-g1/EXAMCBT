package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrBlobNotFound = errors.New("blob not found")

// Blobs stores frames, verification photos and roster photos. Keys are
// slash-separated, e.g. "frames/<exam>/<session>/<unix_ms>.jpg".
type Blobs interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
}

// ValidKey rejects keys that could escape the storage root.
func ValidKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// --- MinIO -----------------------------------------------------------------

type MinIOBlobs struct {
	client *minio.Client
	bucket string
}

// OpenMinIO connects and ensures the bucket exists. The bucket is private: no
// bucket policy is set, so every read goes through the staff-authenticated API.
func OpenMinIO(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOBlobs, error) {
	cl, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	exists, err := cl.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := cl.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
	}
	return &MinIOBlobs{client: cl, bucket: bucket}, nil
}

func (m *MinIOBlobs) Put(ctx context.Context, key string, data []byte, contentType string) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid blob key %q", key)
	}
	_, err := m.client.PutObject(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (m *MinIOBlobs) Get(ctx context.Context, key string) ([]byte, error) {
	if !ValidKey(key) {
		return nil, ErrBlobNotFound
	}
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrBlobNotFound
		}
		return nil, err
	}
	return data, nil
}

func (m *MinIOBlobs) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

func (m *MinIOBlobs) DeletePrefix(ctx context.Context, prefix string) error {
	objects := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	for res := range m.client.RemoveObjects(ctx, m.bucket, objects, minio.RemoveObjectsOptions{}) {
		if res.Err != nil {
			return res.Err
		}
	}
	return nil
}

// --- Filesystem (standalone) -------------------------------------------------

type DiskBlobs struct{ root string }

func OpenDisk(root string) (*DiskBlobs, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &DiskBlobs{root: root}, nil
}

func (d *DiskBlobs) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("invalid blob key %q", key)
	}
	return filepath.Join(d.root, filepath.FromSlash(key)), nil
}

func (d *DiskBlobs) Put(_ context.Context, key string, data []byte, _ string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (d *DiskBlobs) Get(_ context.Context, key string) ([]byte, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, ErrBlobNotFound
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrBlobNotFound
	}
	return data, err
}

func (d *DiskBlobs) Delete(_ context.Context, key string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (d *DiskBlobs) DeletePrefix(_ context.Context, prefix string) error {
	prefix = strings.TrimSuffix(prefix, "/")
	if !ValidKey(prefix) {
		return fmt.Errorf("invalid blob prefix %q", prefix)
	}
	return os.RemoveAll(filepath.Join(d.root, filepath.FromSlash(prefix)))
}
