//go:build conformance

package conformance

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const (
	// startTimeout — сколько ждём, пока сервис начнёт отвечать.
	startTimeout = 10 * time.Second
	// stopTimeout — сколько ждём завершения процесса после SIGTERM.
	stopTimeout = 20 * time.Second
)

// app — запущенный под тестом экземпляр сервиса.
type app struct {
	t          *testing.T
	cmd        *exec.Cmd
	BaseURL    string
	SecretFile string
	dir        string
	exited     chan error
}

// options — что подкрутить в конфиге перед запуском.
type options struct {
	// noPublicLinks оставляет секцию public пустой: сервис обязан отдавать
	// 503 на /public, а не выдумывать ссылки.
	noPublicLinks bool
	// secretFileSize — размер секретного файла. 0 означает «не создавать».
	secretFileSize int
	// shutdownTimeout прокидывается в конфиг.
	shutdownTimeout time.Duration
	// upstream поднимает фиктивный внешний сервис и прописывает его адрес
	// в конфиг. nil означает, что апстрим не настроен.
	upstream http.HandlerFunc
	// upstreamMaxAttempts и upstreamOpenFor настраивают ретраи и breaker.
	upstreamMaxAttempts int
	upstreamTimeout     time.Duration
	upstreamThreshold   int
	upstreamOpenFor     time.Duration
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

// binaryPath возвращает путь к тестируемому бинарю.
func binaryPath(t *testing.T) string {
	t.Helper()
	p := os.Getenv("CONFORMANCE_BINARY")
	if p == "" {
		t.Skip("CONFORMANCE_BINARY is not set — nothing to check")
	}
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("binary %s not found: %v", p, err)
		}
		return p
	}

	// Относительный путь разрешаем от корня репозитория, а не от каталога
	// пакета: go test запускается из conformance/, а бинарь лежит в dist/
	// на уровень выше.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, p)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("binary %q not found in %s or any parent directory", p, mustWd(t))
		}
		dir = parent
	}
}

func orDuration(v, fallback time.Duration) time.Duration {
	if v == 0 {
		return fallback
	}
	return v
}

func orInt(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func mustWd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// start поднимает сервис в изолированном каталоге со сгенерированным конфигом.
func start(t *testing.T, opt options) *app {
	t.Helper()

	bin := binaryPath(t)
	dir := t.TempDir()
	port := freePort(t)

	secretFile := filepath.Join(dir, "secret.test")
	if opt.secretFileSize > 0 {
		if err := os.WriteFile(secretFile, make([]byte, opt.secretFileSize), 0o600); err != nil {
			t.Fatalf("write secret file: %v", err)
		}
	}

	public := `
public:
  discord: "https://discord.gg/conformance"
  chat: "https://t.me/conformance_chat"
  channel: "https://t.me/conformance_channel"
`
	if opt.noPublicLinks {
		public = ""
	}

	shutdown := opt.shutdownTimeout
	if shutdown == 0 {
		shutdown = 10 * time.Second
	}

	upstreamCfg := ""
	if opt.upstream != nil {
		fake := httptest.NewServer(opt.upstream)
		t.Cleanup(fake.Close)
		upstreamCfg = fmt.Sprintf(`
upstream:
  url: %q
  timeout: %s
  maxAttempts: %d
  backoffBase: 20ms
  failureThreshold: %d
  openFor: %s
`,
			fake.URL,
			orDuration(opt.upstreamTimeout, 2*time.Second),
			orInt(opt.upstreamMaxAttempts, 3),
			orInt(opt.upstreamThreshold, 5),
			orDuration(opt.upstreamOpenFor, 5*time.Second),
		)
	}

	cfg := fmt.Sprintf(`prod:
  port: %d
server:
  shutdownTimeout: %s
  readTimeout: 10s
  writeTimeout: 10s
  idleTimeout: 60s
%s
secret:
  chat: "https://t.me/+conformance"
  filePath: %q
  minSize: 2048
%s`, port, shutdown, public, secretFile, upstreamCfg)

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(bin)
	cmd.Dir = dir
	// HOME переопределяем, чтобы сервис не подхватил ~/.testapp/config.yaml
	// разработчика и не выдал ложный результат.
	cmd.Env = append(os.Environ(), "HOME="+dir)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	// Своя группа процессов: если сервис форкается, погасим всё дерево.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}

	a := &app{
		t:          t,
		cmd:        cmd,
		BaseURL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		SecretFile: secretFile,
		dir:        dir,
		exited:     make(chan error, 1),
	}
	go func() { a.exited <- cmd.Wait() }()

	t.Cleanup(a.kill)
	a.waitReady()
	return a
}

// waitReady ждёт, пока сервис начнёт отвечать на /ping.
func (a *app) waitReady() {
	a.t.Helper()
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-a.exited:
			a.t.Fatalf("service exited before becoming ready: %v", err)
		default:
		}

		resp, err := a.client().Get(a.BaseURL + "/ping")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	a.t.Fatalf("service at %s did not start within %s", a.BaseURL, startTimeout)
}

func (a *app) client() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

// get выполняет GET с произвольными заголовками.
func (a *app) get(path string, headers map[string]string) *http.Response {
	a.t.Helper()
	return a.do(http.MethodGet, path, headers)
}

func (a *app) do(method, path string, headers map[string]string) *http.Response {
	a.t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, a.BaseURL+path, nil)
	if err != nil {
		a.t.Fatalf("build request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := a.client().Do(req)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	a.t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// terminate шлёт SIGTERM и ждёт завершения процесса.
// Возвращает false, если процесс не успел завершиться за stopTimeout.
func (a *app) terminate() bool {
	a.t.Helper()
	if err := a.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		a.t.Fatalf("send SIGTERM: %v", err)
	}
	select {
	case <-a.exited:
		return true
	case <-time.After(stopTimeout):
		return false
	}
}

// kill гасит процесс жёстко — страховка на случай, если тест упал раньше.
func (a *app) kill() {
	if a.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-a.cmd.Process.Pid, syscall.SIGKILL)
	select {
	case <-a.exited:
	case <-time.After(5 * time.Second):
	}
}
