//go:build conformance

package conformance

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// C11 — /upstream отключён, пока url не задан. Сервис обязан честно сказать
// об этом, а не падать и не отдавать выдуманный успех.
func TestC11_UpstreamDisabledWhenNotConfigured(t *testing.T) {
	a := start(t, options{secretFileSize: 4096})

	resp := a.get("/upstream", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /upstream = %d, want 503 when upstream.url is empty", resp.StatusCode)
	}
}

// C12 — успешный вызов апстрима.
func TestC12_UpstreamSuccess(t *testing.T) {
	a := start(t, options{
		secretFileSize: 4096,
		upstream: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	})

	resp := a.get("/upstream", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /upstream = %d, want 200", resp.StatusCode)
	}

	var body struct {
		StatusCode int `json:"status_code"`
		Attempts   int `json:"attempts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1 for a healthy upstream", body.Attempts)
	}
}

// C13 — ретраи. Апстрим отдаёт 5xx дважды, затем 200: вызов обязан
// завершиться успехом, а не пробросить первую же ошибку наверх.
func TestC13_UpstreamRetriesOn5xx(t *testing.T) {
	var calls atomic.Int32
	a := start(t, options{
		secretFileSize:      4096,
		upstreamMaxAttempts: 3,
		upstream: func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		},
	})

	resp := a.get("/upstream", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /upstream = %d, want 200 — retries missing?", resp.StatusCode)
	}

	var body struct {
		Attempts int `json:"attempts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Attempts < 2 {
		t.Fatalf("attempts = %d, want at least 2 — no retry happened", body.Attempts)
	}
}

// C14 — 4xx не ретраится: повторять запрос, осознанно отвергнутый апстримом,
// значит зря жечь его бюджет.
func TestC14_UpstreamDoesNotRetry4xx(t *testing.T) {
	var calls atomic.Int32
	a := start(t, options{
		secretFileSize:      4096,
		upstreamMaxAttempts: 5,
		upstream: func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		},
	})

	_ = a.get("/upstream", nil)

	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream was called %d times for a 4xx response, want 1", got)
	}
}

// C15 — бюджет времени распространяется на весь вызов, а не выдаётся заново
// каждой попытке. Иначе пять ретраев по две секунды держат клиента десять.
func TestC15_UpstreamTimeBudgetIsPerCall(t *testing.T) {
	a := start(t, options{
		secretFileSize:      4096,
		upstreamMaxAttempts: 5,
		upstreamTimeout:     500 * time.Millisecond,
		upstream: func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
			w.WriteHeader(http.StatusInternalServerError)
		},
	})

	start := time.Now()
	resp := a.get("/upstream", nil)
	elapsed := time.Since(start)

	if resp.StatusCode < 500 {
		t.Fatalf("GET /upstream = %d, want 5xx for a dead upstream", resp.StatusCode)
	}
	// Пять попыток по 500мс дали бы 2.5с. Общий бюджет держит нас около 500мс.
	if elapsed > 2*time.Second {
		t.Fatalf("call took %s with a 500ms budget — timeout applied per attempt, not per call", elapsed)
	}
}

// C16 — circuit breaker. После серии отказов цепь размыкается: запросы
// перестают уходить в сеть, а клиент получает 503 с Retry-After.
func TestC16_UpstreamCircuitBreakerOpens(t *testing.T) {
	var calls atomic.Int32
	a := start(t, options{
		secretFileSize:      4096,
		upstreamMaxAttempts: 1,
		upstreamThreshold:   2,
		upstreamOpenFor:     30 * time.Second,
		upstreamTimeout:     time.Second,
		upstream: func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		},
	})

	// Добиваем порог отказов.
	for range 3 {
		_ = a.get("/upstream", nil)
	}

	callsBefore := calls.Load()
	resp := a.get("/upstream", nil)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("GET /upstream = %d after repeated failures, want 503 from an open circuit", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("503 from an open circuit has no Retry-After header")
	}
	if calls.Load() != callsBefore {
		t.Fatal("request reached the upstream while the circuit was open")
	}

	// Разомкнутая цепь — это неготовность принимать трафик.
	resp = a.get("/readyz", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /readyz = %d with an open circuit, want 503", resp.StatusCode)
	}
}
