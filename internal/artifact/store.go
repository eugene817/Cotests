// Package artifact stores private problem files outside the public asset tree.
package artifact

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cotests/internal/db"
	"gorm.io/gorm"
)

type Store struct {
	Root          string
	MaxBytes      int64
	MaxTotalBytes int64
}

var storageMu sync.Mutex
var ErrTooLarge = errors.New("artifact exceeds size limit")
var ErrQuota = errors.New("storage quota exceeded")

func (s Store) Ensure() error { return os.MkdirAll(s.Root, 0o700) }

func (s Store) Save(database *gorm.DB, ownerID uint, filename, contentType string, source io.Reader) (*db.Artifact, error) {
	storageMu.Lock()
	defer storageMu.Unlock()
	if ownerID == 0 || strings.TrimSpace(filename) == "" || s.MaxBytes <= 0 {
		return nil, fmt.Errorf("invalid artifact")
	}
	if err := s.Ensure(); err != nil {
		return nil, fmt.Errorf("create artifact directory: %w", err)
	}
	key, err := randomKey()
	if err != nil {
		return nil, err
	}
	temporary, err := os.CreateTemp(s.Root, ".upload-*")
	if err != nil {
		return nil, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(source, s.MaxBytes+1))
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, fmt.Errorf("store artifact: %w", err)
	}
	if written > s.MaxBytes {
		return nil, ErrTooLarge
	}
	if s.MaxTotalBytes > 0 {
		entries, err := os.ReadDir(s.Root)
		if err != nil {
			return nil, err
		}
		var used int64
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".upload-") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			used += info.Size()
		}
		if written > s.MaxTotalBytes-used {
			return nil, ErrQuota
		}
	}
	if err := os.Chmod(temporaryName, 0o600); err != nil {
		return nil, err
	}
	if err := os.Rename(temporaryName, filepath.Join(s.Root, key)); err != nil {
		return nil, fmt.Errorf("commit artifact: %w", err)
	}
	artifact := &db.Artifact{OwnerID: ownerID, StorageKey: key, SHA256: hex.EncodeToString(hash.Sum(nil)), SizeBytes: written, ContentType: contentType, Filename: filepath.Base(filename)}
	if err := database.Create(artifact).Error; err != nil {
		_ = os.Remove(filepath.Join(s.Root, key))
		return nil, err
	}
	return artifact, nil
}

func (s Store) Remove(key string) error {
	if len(key) != 64 || filepath.Base(key) != key {
		return fmt.Errorf("invalid artifact key")
	}
	return os.Remove(filepath.Join(s.Root, key))
}

// CollectTemporary serializes with Save, so active temporary uploads cannot be
// removed. It only cleans staging files left behind by an interrupted process.
func (s Store) CollectTemporary(cutoff time.Time) (int, error) {
	storageMu.Lock()
	defer storageMu.Unlock()
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".upload-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(s.Root, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func (s Store) Open(artifact *db.Artifact) (*os.File, error) {
	if artifact == nil || len(artifact.StorageKey) != 64 || filepath.Base(artifact.StorageKey) != artifact.StorageKey {
		return nil, fmt.Errorf("invalid artifact key")
	}
	return os.Open(filepath.Join(s.Root, artifact.StorageKey))
}

func randomKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
