package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/bear-sre-test-app/internal/config"
)

// testConfig — полностью заполненный конфиг, от которого пляшут тесты.
// Каждый тест правит только то поле, которое проверяет.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Addr:            ":0",
		ShutdownTimeout: time.Second,
		ReadTimeout:     time.Second,
		WriteTimeout:    time.Second,
		IdleTimeout:     time.Second,
		Public: config.PublicLinks{
			Discord: "https://discord.gg/example",
			Chat:    "https://t.me/example_chat",
			Channel: "https://t.me/example_channel",
		},
		Secret: config.SecretConfig{
			Chat:     "https://t.me/+example",
			FilePath: filepath.Join(t.TempDir(), "absent.test"),
			MinSize:  2048,
		},
	}
}

func do(t *testing.T, cfg *config.Config, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	New(cfg).Handler().ServeHTTP(rec, req)
	return rec
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		wantStatus int
	}{
		{"root", http.MethodGet, "/", nil, http.StatusOK},
		{"ping", http.MethodGet, "/ping", nil, http.StatusOK},
		{"public configured", http.MethodGet, "/public", nil, http.StatusOK},
		{"healthz", http.MethodGet, "/healthz", nil, http.StatusOK},
		{"metrics", http.MethodGet, "/metrics", nil, http.StatusOK},
		{"secret without header", http.MethodGet, "/secret", nil, http.StatusUnauthorized},
		{"secret wrong header", http.MethodGet, "/secret", map[string]string{XIamSRE: "dev"}, http.StatusUnauthorized},
		// Файла нет — 503, а не 500: сервис исправен, не готово окружение.
		{"secret no file", http.MethodGet, "/secret", map[string]string{XIamSRE: "SRE"}, http.StatusServiceUnavailable},
		{"upstream disabled", http.MethodGet, "/upstream", nil, http.StatusServiceUnavailable},
		// Раньше catch-all на "/" отдавал 200 на любой путь.
		{"unknown path", http.MethodGet, "/helth", nil, http.StatusNotFound},
		{"unknown nested path", http.MethodGet, "/a/b/c", nil, http.StatusNotFound},
		// ServeMux с методом в шаблоне сам отдаёт 405.
		{"wrong method", http.MethodPost, "/ping", nil, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, testConfig(t), tt.method, tt.path, tt.headers)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestContentTypeIsJSON — Content-Type не выставлялся ни в одной ручке,
// из-за чего curl | jq спотыкался на ровном месте.
func TestContentTypeIsJSON(t *testing.T) {
	paths := []string{"/", "/ping", "/public", "/secret", "/healthz", "/readyz", "/nope"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := do(t, testConfig(t), http.MethodGet, p, nil)
			got := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(got, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
			if !json.Valid(rec.Body.Bytes()) {
				t.Fatalf("body is not valid JSON: %s", rec.Body.String())
			}
		})
	}
}

func TestPublicNotConfigured(t *testing.T) {
	cfg := testConfig(t)
	cfg.Public.Channel = ""

	rec := do(t, cfg, http.MethodGet, "/public", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}

	// Неготовность конфигурации должна отражаться и в readiness.
	rec = do(t, cfg, http.MethodGet, "/readyz", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want 503", rec.Code)
	}
}

func TestSecretSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jtprogru.test")
	if err := os.WriteFile(path, make([]byte, 4096), 0o600); err != nil {
		t.Fatalf("prepare secret file: %v", err)
	}

	cfg := testConfig(t)
	cfg.Secret.FilePath = path

	rec := do(t, cfg, http.MethodGet, "/secret", map[string]string{XIamSRE: "sre"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var got secretResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got.Size != 4096 {
		t.Fatalf("size = %d, want 4096", got.Size)
	}
	if got.Chat != cfg.Secret.Chat {
		t.Fatalf("chat = %q, want %q", got.Chat, cfg.Secret.Chat)
	}
}

// TestSecretHeaderCaseInsensitive фиксирует исторический контракт:
// значение заголовка сравнивается без учёта регистра.
func TestSecretHeaderCaseInsensitive(t *testing.T) {
	for _, v := range []string{"sre", "SRE", "Sre", "sRe"} {
		t.Run(v, func(t *testing.T) {
			rec := do(t, testConfig(t), http.MethodGet, "/secret", map[string]string{XIamSRE: v})
			// Файла нет, поэтому 503 — но не 401: заголовок принят.
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("header %q rejected, want accepted", v)
			}
		})
	}
}

// TestNoFileDescriptorLeak ловит регресс исходной утечки: проверка секретного
// файла не должна оставлять открытых дескрипторов.
func TestNoFileDescriptorLeak(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jtprogru.test")
	if err := os.WriteFile(path, make([]byte, 4096), 0o600); err != nil {
		t.Fatalf("prepare secret file: %v", err)
	}

	before := openFDs(t)
	for range 2000 {
		if _, err := checkSecretFile(path, 2048); err != nil {
			t.Fatalf("checkSecretFile: %v", err)
		}
	}
	after := openFDs(t)

	// Небольшой дрейф допустим (рантайм открывает свои файлы), утечка на
	// 2000 вызовов дала бы рост на тысячи.
	if after-before > 50 {
		t.Fatalf("file descriptors grew from %d to %d — leak suspected", before, after)
	}
}

func openFDs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		t.Skipf("can't count file descriptors on this platform: %v", err)
	}
	return len(entries)
}
