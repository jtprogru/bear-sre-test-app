//go:build conformance

package conformance

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// C1 — HTTP-контракт ручек
// ─────────────────────────────────────────────────────────────────────────────

func TestC1_Endpoints(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		{"root", http.MethodGet, "/", nil, http.StatusOK},
		{"ping", http.MethodGet, "/ping", nil, http.StatusOK},
		{"public", http.MethodGet, "/public", nil, http.StatusOK},
		{"healthz", http.MethodGet, "/healthz", nil, http.StatusOK},
		{"readyz", http.MethodGet, "/readyz", nil, http.StatusOK},
		{"metrics", http.MethodGet, "/metrics", nil, http.StatusOK},
		{"secret authorized", http.MethodGet, "/secret", map[string]string{"X-IAM-SRE": "SRE"}, http.StatusOK},
		{"secret unauthorized", http.MethodGet, "/secret", nil, http.StatusUnauthorized},
		{"secret wrong value", http.MethodGet, "/secret", map[string]string{"X-IAM-SRE": "dev"}, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := a.do(tt.method, tt.path, tt.headers)
			if resp.StatusCode != tt.want {
				t.Fatalf("%s %s = %d, want %d", tt.method, tt.path, resp.StatusCode, tt.want)
			}
		})
	}
}

// C2 — неизвестный путь обязан отдавать 404.
// Исходный catch-all на "/" отвечал 200 на что угодно, из-за чего опечатка
// в адресе выглядела как существующая ручка.
func TestC2_UnknownPathIs404(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	for _, path := range []string{"/helth", "/pong", "/secret/", "/a/b/c", "/PING"} {
		t.Run(path, func(t *testing.T) {
			resp := a.get(path, nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("GET %s = %d, want 404", path, resp.StatusCode)
			}
		})
	}
}

// C3 — неверный метод: 405 и обязательный заголовок Allow (RFC 9110).
func TestC3_WrongMethodIs405(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			resp := a.do(method, "/ping", nil)
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("%s /ping = %d, want 405", method, resp.StatusCode)
			}
			if allow := resp.Header.Get("Allow"); allow == "" {
				t.Fatal("405 response has no Allow header")
			}
		})
	}
}

// C4 — все ответы должны быть валидным JSON с корректным Content-Type.
func TestC4_ResponsesAreJSON(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	paths := []string{"/", "/ping", "/public", "/healthz", "/readyz", "/nonexistent"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			resp := a.get(p, nil)

			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !json.Valid(body) {
				t.Fatalf("body is not valid JSON: %s", body)
			}
		})
	}
}

// C5 — /secret отражает состояние секретного файла.
func TestC5_SecretFileStates(t *testing.T) {
	t.Run("file present", func(t *testing.T) {
		a := start(t, options{secretFileSize: 4096})
		resp := a.get("/secret", map[string]string{"X-IAM-SRE": "sre"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}

		var body struct {
			Chat string `json:"chat"`
			Size int64  `json:"size"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Size != 4096 {
			t.Fatalf("size = %d, want 4096", body.Size)
		}
		if body.Chat == "" {
			t.Fatal("chat link is empty")
		}
	})

	t.Run("file absent", func(t *testing.T) {
		a := start(t, options{})
		resp := a.get("/secret", map[string]string{"X-IAM-SRE": "sre"})
		if resp.StatusCode < 500 {
			t.Fatalf("status = %d, want 5xx when secret file is missing", resp.StatusCode)
		}
	})

	t.Run("file too short", func(t *testing.T) {
		a := start(t, options{secretFileSize: 100})
		resp := a.get("/secret", map[string]string{"X-IAM-SRE": "sre"})
		if resp.StatusCode < 500 {
			t.Fatalf("status = %d, want 5xx when secret file is too short", resp.StatusCode)
		}
	})
}

// C6 — /public обязан честно сообщать о неполной конфигурации, а не
// подставлять захардкоженные ссылки.
func TestC6_PublicRequiresConfig(t *testing.T) {
	a := start(t, options{noPublicLinks: true, secretFileSize: 4096})

	resp := a.get("/public", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /public = %d, want 503 when links are not configured", resp.StatusCode)
	}

	// Неготовность конфигурации обязана отражаться в readiness.
	resp = a.get("/readyz", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz = %d, want 503 when config is incomplete", resp.StatusCode)
	}

	// А liveness — нет: процесс жив, перезапускать его незачем.
	resp = a.get("/healthz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200 — liveness must not depend on config", resp.StatusCode)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// C7 — метрики
// ─────────────────────────────────────────────────────────────────────────────

func TestC7_MetricsExposed(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	// Прогреваем, чтобы счётчики точно появились.
	for range 5 {
		_ = a.get("/ping", nil)
	}

	body := readAll(t, a.get("/metrics", nil))
	required := []string{
		"http_requests_total",
		"http_request_duration_seconds",
	}
	for _, name := range required {
		if !strings.Contains(body, name) {
			t.Errorf("metric %q is missing from /metrics", name)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// C8 — устойчивость: таймауты и утечки
// ─────────────────────────────────────────────────────────────────────────────

// C8 — Slowloris. Открываем соединение и не досылаем заголовки. Сервис обязан
// разорвать его по ReadHeaderTimeout, а не держать вечно (gosec G112).
func TestC8_SlowlorisConnectionIsDropped(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	addr := strings.TrimPrefix(a.BaseURL, "http://")
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Отправляем начало запроса и намеренно не завершаем заголовки.
	if _, err := conn.Write([]byte("GET /ping HTTP/1.1\r\nHost: localhost\r\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Ждём дольше разумного ReadHeaderTimeout, но меньше вечности.
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	buf := make([]byte, 256)
	_, err = conn.Read(buf)

	// Ожидаем одно из двух: сервер закрыл соединение (EOF/reset) или ответил
	// 408 Request Timeout. Наш собственный read deadline означает, что сервер
	// продолжает держать полуоткрытое соединение, — это и есть Slowloris.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		t.Fatal("server kept a half-open connection for 30s — ReadHeaderTimeout is missing")
	}
	if err == nil && !strings.Contains(string(buf), "408") {
		t.Fatalf("connection with incomplete headers was answered with %q, want close or 408", strings.TrimSpace(string(buf)))
	}
}

// C9 — утечка файловых дескрипторов. Ручка /secret в исходной реализации
// открывала файл и не закрывала его: несколько тысяч запросов исчерпывали
// ulimit -n. Считаем process_open_fds до и после нагрузки.
func TestC9_NoFileDescriptorLeak(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	before, ok := processOpenFDs(t, a)
	if !ok {
		t.Skip("process_open_fds is not exported on this platform")
	}

	const requests = 3000
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}
			for range requests / 16 {
				req, _ := http.NewRequest(http.MethodGet, a.BaseURL+"/secret", nil)
				req.Header.Set("X-IAM-SRE", "sre")
				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// Даём рантайму время закрыть keep-alive соединения.
	time.Sleep(2 * time.Second)

	after, _ := processOpenFDs(t, a)
	t.Logf("open fds: %.0f before, %.0f after %d requests", before, after, requests)

	// Утечка по дескриптору на запрос дала бы рост на тысячи.
	// Порог с запасом на пул соединений.
	if after-before > 200 {
		t.Fatalf("open fds grew from %.0f to %.0f after %d requests — descriptor leak", before, after, requests)
	}
}

// C10 — graceful shutdown. По SIGTERM сервис обязан дождаться запроса
// в полёте и завершиться сам, не по kill -9.
func TestC10_GracefulShutdown(t *testing.T) {
	a := start(t, options{secretFileSize: 4096, shutdownTimeout: 10 * time.Second})

	// Запрос в полёте.
	respCh := make(chan int, 1)
	go func() {
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(a.BaseURL + "/ping")
		if err != nil {
			respCh <- 0
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(io.Discard, resp.Body)
		respCh <- resp.StatusCode
	}()

	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	if !a.terminate() {
		t.Fatalf("process did not exit within %s after SIGTERM — no graceful shutdown", stopTimeout)
	}
	elapsed := time.Since(start)
	t.Logf("process exited %s after SIGTERM", elapsed)

	select {
	case code := <-respCh:
		if code != http.StatusOK {
			t.Fatalf("in-flight request got %d, want 200 — connection was dropped during shutdown", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request never completed")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

var openFDsRe = regexp.MustCompile(`(?m)^process_open_fds\s+([0-9.e+]+)`)

func processOpenFDs(t *testing.T, a *app) (float64, bool) {
	t.Helper()
	m := openFDsRe.FindStringSubmatch(readAll(t, a.get("/metrics", nil)))
	if len(m) != 2 {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}
