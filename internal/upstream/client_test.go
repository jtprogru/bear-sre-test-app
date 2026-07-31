package upstream

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func opts(url string) Options {
	return Options{
		URL:              url,
		Timeout:          2 * time.Second,
		MaxAttempts:      3,
		BackoffBase:      time.Millisecond,
		FailureThreshold: 2,
		OpenFor:          time.Minute,
	}
}

func TestDoSuccessFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res, err := New(opts(srv.URL)).Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", res.Attempts)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

// TestRetryOn5xx: 5xx ретраится, и вызов успевает завершиться успехом.
func TestRetryOn5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res, err := New(opts(srv.URL)).Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if res.Attempts != 3 {
		t.Fatalf("attempts = %d, want 3", res.Attempts)
	}
}

// TestNoRetryOn4xx: повторять запрос, осознанно отвергнутый апстримом,
// бессмысленно — 4xx возвращается с первой попытки.
func TestNoRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	res, err := New(opts(srv.URL)).Do(context.Background())
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream called %d times, want 1", got)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

// TestBudgetIsPerCallNotPerAttempt — бюджет времени распространяется на весь
// вызов вместе с ретраями, а не выдаётся заново каждой попытке.
func TestBudgetIsPerCallNotPerAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	o := opts(srv.URL)
	o.Timeout = 200 * time.Millisecond
	o.MaxAttempts = 5

	start := time.Now()
	_, err := New(o).Do(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Пять попыток по 200мс дали бы секунду. Общий бюджет держит нас у 200мс.
	if elapsed > time.Second {
		t.Fatalf("elapsed = %v, want under 1s — budget applied per attempt?", elapsed)
	}
}

// TestCircuitOpensAndBlocks: после серии отказов цепь размыкается и запросы
// перестают уходить в сеть.
func TestCircuitOpensAndBlocks(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	o := opts(srv.URL)
	o.FailureThreshold = 1
	c := New(o)

	if _, err := c.Do(context.Background()); err == nil {
		t.Fatal("expected failure on first call")
	}
	if c.State() != StateOpen {
		t.Fatalf("state = %v, want open", c.State())
	}

	callsAfterOpen := calls.Load()
	_, err := c.Do(context.Background())
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
	if calls.Load() != callsAfterOpen {
		t.Fatal("request reached upstream while circuit was open")
	}
}

// TestCircuitHalfOpenRecovers: по истечении cooldown цепь пускает пробный
// запрос и, если он успешен, замыкается обратно.
func TestCircuitHalfOpenRecovers(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	o := opts(srv.URL)
	o.FailureThreshold = 1
	o.OpenFor = time.Minute
	c := New(o)

	// Управляем временем вручную, чтобы не спать в тесте минуту.
	now := time.Now()
	c.now = func() time.Time { return now }

	if _, err := c.Do(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	if c.State() != StateOpen {
		t.Fatalf("state = %v, want open", c.State())
	}

	now = now.Add(2 * time.Minute)
	if c.State() != StateHalfOpen {
		t.Fatalf("state = %v, want half-open after cooldown", c.State())
	}

	fail.Store(false)
	if _, err := c.Do(context.Background()); err != nil {
		t.Fatalf("probe call failed: %v", err)
	}
	if c.State() != StateClosed {
		t.Fatalf("state = %v, want closed after successful probe", c.State())
	}
}

// TestBackoffHasJitter: без джиттера все реплики ретраят синхронно и добивают
// апстрим. Проверяем, что паузы не совпадают между собой.
func TestBackoffHasJitter(t *testing.T) {
	c := New(opts("http://example.invalid"))
	seen := map[time.Duration]int{}
	for range 200 {
		seen[c.backoff(4)]++
	}
	if len(seen) < 10 {
		t.Fatalf("only %d distinct backoff values — jitter missing?", len(seen))
	}
}

// TestCallerCancellationStopsRetries: если клиент отвалился, продолжать
// ретраить в апстрим незачем.
func TestCallerCancellationStopsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	o := opts(srv.URL)
	o.MaxAttempts = 50
	o.BackoffBase = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if _, err := New(o).Do(ctx); err == nil {
		t.Fatal("expected error after cancellation")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("elapsed = %v — retries continued after cancellation", elapsed)
	}
}
