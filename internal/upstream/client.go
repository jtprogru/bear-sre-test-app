// Package upstream — HTTP-клиент к внешнему сервису с бюджетом времени,
// ретраями и circuit breaker'ом.
//
// Три правила, вокруг которых всё построено:
//
//   - бюджет времени задаётся один раз на весь вызов, а не на попытку;
//     иначе три ретрая по 2с превращаются в 6с ожидания для клиента;
//   - backoff обязательно с джиттером, иначе после сетевого сбоя все реплики
//     ретраят синхронно и добивают апстрим;
//   - breaker размыкает цепь после серии отказов, чтобы не тратить бюджет
//     на заведомо мёртвый сервис.
package upstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/jtprogru/bear-sre-test-app/internal/metrics"
)

// ErrCircuitOpen возвращается, пока breaker разомкнут: запрос не уходит в сеть.
var ErrCircuitOpen = errors.New("upstream circuit breaker is open")

// State — состояние circuit breaker'а.
type State int

const (
	StateClosed   State = iota // пропускаем всё
	StateHalfOpen              // пропускаем одну пробную попытку
	StateOpen                  // не пропускаем ничего
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// Options — настройки клиента.
type Options struct {
	URL              string
	Timeout          time.Duration // бюджет на весь вызов, включая ретраи
	MaxAttempts      int
	BackoffBase      time.Duration
	FailureThreshold int           // отказов подряд до размыкания цепи
	OpenFor          time.Duration // сколько цепь остаётся разомкнутой
}

// Client — потокобезопасный клиент к одному апстриму.
type Client struct {
	opts Options
	http *http.Client

	// now подменяется в тестах, чтобы не спать реальное время.
	now func() time.Time

	mu          sync.Mutex
	state       State
	failures    int
	openedUntil time.Time
}

// Result — итог вызова апстрима.
type Result struct {
	StatusCode int           `json:"status_code"`
	Attempts   int           `json:"attempts"`
	Elapsed    time.Duration `json:"-"`
	ElapsedMs  int64         `json:"elapsed_ms"`
}

// New создаёт клиента. Транспорт свой, а не http.DefaultClient: у дефолтного
// нет таймаутов и он общий на весь процесс.
func New(opts Options) *Client {
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 1
	}
	if opts.BackoffBase <= 0 {
		opts.BackoffBase = 50 * time.Millisecond
	}
	if opts.FailureThreshold < 1 {
		opts.FailureThreshold = 5
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 32
	transport.ResponseHeaderTimeout = opts.Timeout

	c := &Client{
		opts: opts,
		http: &http.Client{Transport: transport},
		now:  time.Now,
	}
	metrics.CircuitBreakerState.Set(float64(StateClosed))
	return c
}

// State возвращает текущее состояние breaker'а с учётом истёкшего cooldown.
func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Client) stateLocked() State {
	if c.state == StateOpen && !c.now().Before(c.openedUntil) {
		c.state = StateHalfOpen
		metrics.CircuitBreakerState.Set(float64(StateHalfOpen))
	}
	return c.state
}

// allow решает, пропускать ли вызов в сеть.
func (c *Client) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked() != StateOpen
}

func (c *Client) onSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures = 0
	c.state = StateClosed
	metrics.CircuitBreakerState.Set(float64(StateClosed))
}

func (c *Client) onFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures++
	// В half-open одного отказа достаточно, чтобы снова разомкнуть цепь.
	if c.state == StateHalfOpen || c.failures >= c.opts.FailureThreshold {
		c.state = StateOpen
		c.openedUntil = c.now().Add(c.opts.OpenFor)
		metrics.CircuitBreakerState.Set(float64(StateOpen))
	}
}

// backoff считает паузу перед попыткой attempt (нумерация с 1).
// Экспонента с полным джиттером: пауза равномерна на [0, base*2^(attempt-1)].
func (c *Client) backoff(attempt int) time.Duration {
	shift := attempt - 1
	if shift > 10 { // защита от переполнения на длинных сериях
		shift = 10
	}
	maxDelay := c.opts.BackoffBase * (1 << shift)
	// #nosec G404 -- джиттер не криптографический: нужна только развязка
	// синхронных ретраев между репликами, crypto/rand здесь избыточен.
	return time.Duration(rand.Int64N(int64(maxDelay) + 1))
}

// Do выполняет запрос к апстриму. Возвращает ErrCircuitOpen, если цепь
// разомкнута, context.DeadlineExceeded — если исчерпан бюджет времени.
func (c *Client) Do(ctx context.Context) (Result, error) {
	start := c.now()
	defer func() { metrics.UpstreamDuration.Observe(time.Since(start).Seconds()) }()

	if !c.allow() {
		metrics.UpstreamRequestsTotal.WithLabelValues("circuit_open").Inc()
		return Result{}, ErrCircuitOpen
	}

	// Бюджет — на весь вызов целиком, а не на каждую попытку.
	ctx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= c.opts.MaxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				c.onFailure()
				metrics.UpstreamRequestsTotal.WithLabelValues("failure").Inc()
				return Result{Attempts: attempt - 1}, ctx.Err()
			case <-time.After(c.backoff(attempt)):
			}
		}

		metrics.UpstreamAttemptsTotal.Inc()
		code, err := c.attempt(ctx)
		if err == nil {
			c.onSuccess()
			metrics.UpstreamRequestsTotal.WithLabelValues("success").Inc()
			elapsed := time.Since(start)
			return Result{
				StatusCode: code,
				Attempts:   attempt,
				Elapsed:    elapsed,
				ElapsedMs:  elapsed.Milliseconds(),
			}, nil
		}
		lastErr = err

		// Бюджет исчерпан — дальнейшие попытки бессмысленны.
		if ctx.Err() != nil {
			break
		}
	}

	c.onFailure()
	metrics.UpstreamRequestsTotal.WithLabelValues("failure").Inc()
	return Result{Attempts: c.opts.MaxAttempts}, lastErr
}

// attempt — одна попытка. 5xx считается ошибкой и ретраится, 4xx — нет:
// повторять запрос, который апстрим осознанно отверг, бессмысленно.
func (c *Client) attempt(ctx context.Context) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.opts.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("upstream call: %w", err)
	}
	defer func() {
		// Тело нужно дочитать и закрыть, иначе соединение не вернётся в пул.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= http.StatusInternalServerError {
		return resp.StatusCode, fmt.Errorf("upstream returned %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
