package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Runtime struct {
	DataDir          string
	ArtifactDir      string
	DatabaseDSN      string
	ListenAddress    string
	PublicURL        *url.URL
	SecureCookies    bool
	MaxArtifactBytes int64
	MaxImportBytes   int64
	MaxImportFiles   int
	MaxStorageBytes  int64
}

// Load reads the small set of deployment settings used by the binary.
// DATABASE_URL takes precedence so an existing SQLite or PostgreSQL deployment
// keeps its configured database when DATA_DIR is introduced.
func Load(lookup func(string) string) (Runtime, error) {
	dataDir := strings.TrimSpace(lookup("DATA_DIR"))
	if dataDir == "" {
		dataDir = "."
	}
	dataDir = filepath.Clean(dataDir)
	artifactDir := strings.TrimSpace(lookup("ARTIFACT_DIR"))
	if artifactDir == "" {
		artifactDir = filepath.Join(dataDir, "artifacts")
	}
	artifactDir = filepath.Clean(artifactDir)
	maxArtifactBytes, err := positiveInt64(lookup("MAX_ARTIFACT_BYTES"), 10<<20)
	if err != nil {
		return Runtime{}, fmt.Errorf("MAX_ARTIFACT_BYTES: %w", err)
	}
	maxImportBytes, err := positiveInt64(lookup("MAX_IMPORT_BYTES"), 50<<20)
	if err != nil {
		return Runtime{}, fmt.Errorf("MAX_IMPORT_BYTES: %w", err)
	}
	maxImportFiles, err := positiveInt(lookup("MAX_IMPORT_FILES"), 100)
	if err != nil {
		return Runtime{}, fmt.Errorf("MAX_IMPORT_FILES: %w", err)
	}
	maxStorageBytes, err := positiveInt64(lookup("MAX_STORAGE_BYTES"), 1<<30)
	if err != nil {
		return Runtime{}, fmt.Errorf("MAX_STORAGE_BYTES: %w", err)
	}
	// Bound in-memory ZIP/HTTP parsing and leave room for multipart overhead.
	if maxArtifactBytes > 512<<20 || maxImportBytes > 512<<20 || maxImportFiles > 10000 {
		return Runtime{}, fmt.Errorf("upload limits exceed supported bounds (512 MiB / 10000 files)")
	}

	dsn := strings.TrimSpace(lookup("DATABASE_URL"))
	if dsn == "" {
		dsn = filepath.Join(dataDir, "cotests.db")
	}

	publicURL, err := parsePublicURL(lookup("PUBLIC_URL"))
	if err != nil {
		return Runtime{}, err
	}
	listenAddress := strings.TrimSpace(lookup("LISTEN_ADDR"))
	if listenAddress == "" {
		listenAddress = ":3000"
	}

	secureCookies := strings.EqualFold(strings.TrimSpace(lookup("SECURE_COOKIES")), "true")
	if publicURL != nil && publicURL.Scheme == "https" {
		secureCookies = true
	}

	return Runtime{
		DataDir:          dataDir,
		ArtifactDir:      artifactDir,
		DatabaseDSN:      dsn,
		ListenAddress:    listenAddress,
		PublicURL:        publicURL,
		SecureCookies:    secureCookies,
		MaxArtifactBytes: maxArtifactBytes,
		MaxImportBytes:   maxImportBytes,
		MaxImportFiles:   maxImportFiles,
		MaxStorageBytes:  maxStorageBytes,
	}, nil
}

func EnsureDataDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect data directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("data directory %q is not a directory", path)
	}
	return nil
}

func positiveInt64(raw string, fallback int64) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("must be a positive integer")
	}
	return value, nil
}

func positiveInt(raw string, fallback int) (int, error) {
	value, err := positiveInt64(raw, int64(fallback))
	if err != nil {
		return 0, err
	}
	if value > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("is too large")
	}
	return int(value), nil
}

func parsePublicURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("PUBLIC_URL must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("PUBLIC_URL must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return nil, fmt.Errorf("PUBLIC_URL must contain only an origin")
	}
	parsed.Path = ""
	return parsed, nil
}
