// Package importer validates bounded, non-executable problem content imports.
package importer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type Limits struct {
	MaxBytes int64
	MaxFiles int
}

type TestPair struct {
	Name   string
	Input  []byte
	Output []byte
}

// ReadTestZIP accepts only flat matching .in/.out pairs. It deliberately does
// not interpret legacy XML or execute a checker.
func ReadTestZIP(source io.Reader, limits Limits) ([]TestPair, error) {
	if limits.MaxBytes <= 0 || limits.MaxFiles <= 0 {
		return nil, fmt.Errorf("invalid import limits")
	}
	archiveBytes, err := io.ReadAll(io.LimitReader(source, limits.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(archiveBytes)) > limits.MaxBytes {
		return nil, fmt.Errorf("archive exceeds %d byte limit", limits.MaxBytes)
	}
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, fmt.Errorf("invalid ZIP archive: %w", err)
	}
	if len(reader.File) == 0 {
		return nil, fmt.Errorf("archive must contain tests")
	}
	if len(reader.File) > limits.MaxFiles {
		return nil, fmt.Errorf("archive has more than %d files", limits.MaxFiles)
	}
	files := make(map[string]map[string][]byte)
	var total int64
	for _, file := range reader.File {
		if err := validName(file.Name); err != nil {
			return nil, err
		}
		if file.Mode()&os.ModeType != 0 {
			return nil, fmt.Errorf("only regular files are allowed")
		}
		ext := strings.ToLower(path.Ext(file.Name))
		if ext != ".in" && ext != ".out" {
			return nil, fmt.Errorf("unsupported file %q", file.Name)
		}
		base := strings.TrimSuffix(file.Name, path.Ext(file.Name))
		if files[base] == nil {
			files[base] = make(map[string][]byte)
		}
		if _, exists := files[base][ext]; exists {
			return nil, fmt.Errorf("duplicate file %q", file.Name)
		}
		content, err := readBounded(file, limits.MaxBytes-total)
		if err != nil {
			return nil, err
		}
		total += int64(len(content))
		if total > limits.MaxBytes {
			return nil, fmt.Errorf("archive contents exceed %d byte limit", limits.MaxBytes)
		}
		files[base][ext] = content
	}
	names := make([]string, 0, len(files))
	for name, pair := range files {
		if pair[".in"] == nil || pair[".out"] == nil {
			return nil, fmt.Errorf("%q must have matching .in and .out files", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]TestPair, 0, len(names))
	for _, name := range names {
		result = append(result, TestPair{Name: name, Input: files[name][".in"], Output: files[name][".out"]})
	}
	return result, nil
}

func validName(name string) error {
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) || strings.Contains(path.Clean(name), "../") || path.Base(name) != name {
		return fmt.Errorf("unsafe file name %q", name)
	}
	return nil
}

func readBounded(file *zip.File, remaining int64) ([]byte, error) {
	if remaining < 0 {
		return nil, fmt.Errorf("archive contents exceed limit")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, remaining+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > remaining {
		return nil, fmt.Errorf("archive contents exceed limit")
	}
	return data, nil
}
