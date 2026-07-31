package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSecretFile(t *testing.T) {
	dir := t.TempDir()

	write := func(name string, size int) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return p
	}

	subdir := filepath.Join(dir, "as-directory")
	if err := os.Mkdir(subdir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	tests := []struct {
		name     string
		path     string
		minSize  int64
		wantSize int64
		wantErr  error
	}{
		{"absent", filepath.Join(dir, "nope"), 2048, 0, ErrSecretFileNotFound},
		{"directory", subdir, 2048, 0, ErrSecretFileNotFound},
		{"empty", write("empty", 0), 2048, 0, ErrSecretFileIsEmpty},
		{"too short", write("short", 100), 2048, 0, ErrSecretFileIsTooShort},
		{"exactly min size", write("exact", 2048), 2048, 2048, nil},
		{"large enough", write("large", 4096), 2048, 4096, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, err := checkSecretFile(tt.path, tt.minSize)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if size != tt.wantSize {
				t.Fatalf("size = %d, want %d", size, tt.wantSize)
			}
		})
	}
}
