package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Runtime struct {
	DataDir       string
	DatabaseDSN   string
	ListenAddress string
	PublicURL     *url.URL
	SecureCookies bool
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
		DataDir:       dataDir,
		DatabaseDSN:   dsn,
		ListenAddress: listenAddress,
		PublicURL:     publicURL,
		SecureCookies: secureCookies,
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
