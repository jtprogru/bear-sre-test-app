package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// chdir переводит тест в отдельный каталог: конфиг ищется в "." первым,
// поэтому каждый тест получает изолированное окружение.
func chdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	// Гасим внешние каталоги поиска, чтобы тест не зацепил реальный
	// /etc/testapp или ~/.testapp на машине разработчика.
	t.Setenv("HOME", dir)
	return dir
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestMissingConfig(t *testing.T) {
	chdir(t)

	_, err := New()
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("err = %v, want ErrConfigNotFound", err)
	}
}

func TestPublicConfigured(t *testing.T) {
	tests := []struct {
		name  string
		links PublicLinks
		want  bool
	}{
		{"all set", PublicLinks{"d", "c", "ch"}, true},
		{"discord missing", PublicLinks{"", "c", "ch"}, false},
		{"chat missing", PublicLinks{"d", "", "ch"}, false},
		{"channel missing", PublicLinks{"d", "c", ""}, false},
		{"empty", PublicLinks{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.links.Configured(); got != tt.want {
				t.Fatalf("Configured() = %v, want %v", got, tt.want)
			}
		})
	}
}
