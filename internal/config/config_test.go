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

func TestMissingConfigReportsSearchPaths(t *testing.T) {
	chdir(t)

	_, err := New()
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("err = %v, want ErrConfigNotFound", err)
	}
	// Прежняя реализация писала «can't load config» без единой подробности.
	// Ошибка обязана называть каталоги, в которых искали.
	for _, p := range SearchPaths() {
		if !contains(err.Error(), p) {
			t.Fatalf("error %q does not mention search path %q", err, p)
		}
	}
}

func TestBrokenYAMLIsDistinctFromMissing(t *testing.T) {
	dir := chdir(t)
	writeConfig(t, dir, "prod:\n  port: [not a number\n")

	_, err := New()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if errors.Is(err, ErrConfigNotFound) {
		t.Fatal("broken YAML reported as missing file")
	}
}

func TestPortFromFile(t *testing.T) {
	dir := chdir(t)
	writeConfig(t, dir, "prod:\n  port: 9091\n")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.Addr != ":9091" {
		t.Fatalf("addr = %q, want :9091", cfg.Addr)
	}
}

// TestEnvOverridesFile — окружение должно перекрывать файл. Раньше SRV_ADDR
// объявлялся в .env, Dockerfile и compose, но код его не читал вовсе.
func TestEnvOverridesFile(t *testing.T) {
	dir := chdir(t)
	writeConfig(t, dir, "prod:\n  port: 9091\n")
	t.Setenv("SRV_ADDR", "8123")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.Addr != ":8123" {
		t.Fatalf("addr = %q, want :8123 (SRV_ADDR ignored?)", cfg.Addr)
	}
}

func TestEnvOnlyWithoutFile(t *testing.T) {
	chdir(t)
	t.Setenv("SRV_ADDR", ":8124")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cfg.Addr != ":8124" {
		t.Fatalf("addr = %q, want :8124", cfg.Addr)
	}
}

// TestInvalidPortRejected: прежняя реализация при отсутствующем ключе
// молча получала 0 и поднимала сервер на случайном порту.
func TestInvalidPortRejected(t *testing.T) {
	for _, body := range []string{"prod:\n  port: 0\n", "prod:\n  port: 70000\n", "tg:\n  chatBirthDate: x\n"} {
		t.Run(body, func(t *testing.T) {
			dir := chdir(t)
			writeConfig(t, dir, body)
			t.Setenv("SRV_ADDR", "")

			cfg, err := New()
			if body == "tg:\n  chatBirthDate: x\n" {
				// Ключа prod.port нет — работает дефолт 8080, это валидно.
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				if cfg.Addr != ":8080" {
					t.Fatalf("addr = %q, want default :8080", cfg.Addr)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error for %q, got addr %q", body, cfg.Addr)
			}
		})
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

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
