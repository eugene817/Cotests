package importer

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

func TestReadTestZIPValidatesPairsAndPaths(t *testing.T) {
	makeZIP := func(files map[string]string) []byte {
		var result bytes.Buffer
		writer := zip.NewWriter(&result)
		for name, content := range files {
			entry, _ := writer.Create(name)
			_, _ = entry.Write([]byte(content))
		}
		_ = writer.Close()
		return result.Bytes()
	}
	pairs, err := ReadTestZIP(bytes.NewReader(makeZIP(map[string]string{"01.in": "1", "01.out": "2"})), Limits{MaxBytes: 1024, MaxFiles: 2})
	if err != nil || len(pairs) != 1 || string(pairs[0].Output) != "2" {
		t.Fatalf("pairs = %#v, err = %v", pairs, err)
	}
	if _, err := ReadTestZIP(bytes.NewReader(makeZIP(map[string]string{"../bad.in": "1", "../bad.out": "2"})), Limits{MaxBytes: 1024, MaxFiles: 2}); err == nil {
		t.Fatal("unsafe path accepted")
	}
	if _, err := ReadTestZIP(bytes.NewReader(makeZIP(map[string]string{"one.in": "1"})), Limits{MaxBytes: 1024, MaxFiles: 2}); err == nil {
		t.Fatal("unpaired input accepted")
	}
}

func TestReadTestZIPRejectsSpecialFilesDuplicatesAndExpansion(t *testing.T) {
	for _, test := range []struct {
		name  string
		files []string
		mode  os.FileMode
		data  string
		limit int64
	}{
		{"symlink", []string{"a.in", "a.out"}, os.ModeSymlink | 0o777, "target", 4096},
		{"duplicate", []string{"a.in", "a.in", "a.out"}, 0o600, "x", 4096},
		{"expanded", []string{"a.in", "a.out"}, 0o600, string(bytes.Repeat([]byte("x"), 10000)), 4096},
		{"absolute", []string{"/a.in", "/a.out"}, 0o600, "x", 4096},
		{"windows traversal", []string{`..\a.in`, `..\a.out`}, 0o600, "x", 4096},
	} {
		t.Run(test.name, func(t *testing.T) {
			var raw bytes.Buffer
			writer := zip.NewWriter(&raw)
			for _, name := range test.files {
				header := &zip.FileHeader{Name: name, Method: zip.Deflate}
				header.SetMode(test.mode)
				file, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = file.Write([]byte(test.data))
			}
			_ = writer.Close()
			if _, err := ReadTestZIP(bytes.NewReader(raw.Bytes()), Limits{MaxBytes: test.limit, MaxFiles: 10}); err == nil {
				t.Fatal("invalid ZIP accepted")
			}
		})
	}
}
