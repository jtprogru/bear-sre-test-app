package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// freePort занимает случайный свободный порт и сразу отдаёт его: сервер в
// тестах не должен садиться на фиксированный порт.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return port
}

// TestTimeoutsAreSet — без таймаутов сервис ложится от Slowloris (gosec G112).
// Проверяем, что ни один из них не остался нулевым.
func TestTimeoutsAreSet(t *testing.T) {
	s := New(testConfig(t))

	tests := []struct {
		name  string
		value time.Duration
	}{
		{"ReadHeaderTimeout", s.srv.ReadHeaderTimeout},
		{"ReadTimeout", s.srv.ReadTimeout},
		{"WriteTimeout", s.srv.WriteTimeout},
		{"IdleTimeout", s.srv.IdleTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value <= 0 {
				t.Fatalf("%s = %v, want positive", tt.name, tt.value)
			}
		})
	}
	if s.srv.MaxHeaderBytes <= 0 {
		t.Fatal("MaxHeaderBytes is unset")
	}
}

// TestGracefulShutdown: по отмене контекста сервер обязан дождаться
// завершения текущего запроса, а не оборвать соединение.
func TestGracefulShutdown(t *testing.T) {
	cfg := testConfig(t)
	cfg.Addr = fmt.Sprintf("127.0.0.1:%d", freePort(t))
	cfg.ShutdownTimeout = 5 * time.Second

	s := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()

	base := "http://" + cfg.Addr
	waitReady(t, base+"/healthz")

	// Отправляем запрос и гасим сервер, пока запрос в полёте.
	respCh := make(chan int, 1)
	go func() {
		resp, err := http.Get(base + "/ping") //nolint:noctx // короткий тестовый вызов
		if err != nil {
			respCh <- 0
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respCh <- resp.StatusCode
	}()

	cancel()

	select {
	case code := <-respCh:
		if code != http.StatusOK && code != 0 {
			t.Fatalf("in-flight request got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request never completed")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop within shutdown timeout")
	}

	// После остановки порт обязан быть свободен.
	if _, err := http.Get(base + "/ping"); err == nil { //nolint:noctx // короткий тестовый вызов
		t.Fatal("server still accepts connections after shutdown")
	}
}

// TestReadinessDropsBeforeShutdown: readiness обязан сняться раньше, чем
// сервер перестанет принимать соединения, иначе балансировщик не успеет
// увести трафик и клиенты словят обрывы.
func TestReadinessDropsBeforeShutdown(t *testing.T) {
	s := New(testConfig(t))

	rec := do(t, testConfig(t), http.MethodGet, "/readyz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz before shutdown = %d, want 200", rec.Code)
	}

	s.ready.Store(false)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec2, req)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz during shutdown = %d, want 503", rec2.Code)
	}
}

func waitReady(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:noctx // короткий тестовый вызов
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s never became ready", url)
}
