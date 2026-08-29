package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/jtprogru/bear-sre-test-app/internal/upstream"
)

// writeJSON — единственная точка записи ответа. Раньше JSON собирался
// через fmt.Sprintf с подстановкой текста ошибки: любая кавычка внутри
// ошибки ломала тело ответа. Плюс здесь же выставляется Content-Type,
// которого не было ни в одной ручке.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		// Маршалинг статических структур упасть не может, но если это
		// произошло — отдаём валидный JSON, а не полуготовый ответ.
		log.Error().Err(err).Msg("can't marshal response payload")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"msg":"internal error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(append(body, '\n')); err != nil {
		log.Error().Err(err).Msg("can't write response body")
	}
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, msgResponse{Msg: "This is home page"})
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, msgResponse{Msg: "pong"})
}

// handleNotFound отвечает на всё, что не подошло ни одному маршруту.
// Прежний catch-all на "/" отдавал 200 и домашнюю страницу на любой путь,
// из-за чего опечатка в адресе выглядела как существующая ручка.
func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, errorResponse{Msg: ErrNotFound.Error()})
}

// methodNotAllowed отвечает 405 и обязательным по RFC 9110 заголовком Allow:
// без него клиент не знает, каким методом ручку всё-таки звать.
func methodNotAllowed(allowed ...string) http.Handler {
	allow := strings.Join(allowed, ", ")
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Msg: ErrMethodNotAllowed.Error()})
	})
}

// handlePublic отдаёт публичные ссылки. Если конфиг неполон — 503:
// это и есть обещанная README проверка «правильно выставленного параметра».
func (s *Server) handlePublic(w http.ResponseWriter, _ *http.Request) {
	if !s.cfg.Public.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Msg: ErrPublicNotConfigured.Error()})
		return
	}

	writeJSON(w, http.StatusOK, publicResponse{
		Chat:    s.cfg.Public.Chat,
		Channel: s.cfg.Public.Channel,
	})
}

func (s *Server) handleSecret(w http.ResponseWriter, r *http.Request) {
	if !hasSREHeader(r) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Msg: ErrHeaderXIamSRENotSet.Error()})
		return
	}

	size, err := checkSecretFile(s.cfg.Secret.FilePath, s.cfg.Secret.MinSize)
	if err != nil {
		// Отсутствие файла — это состояние окружения, а не поломка сервиса,
		// поэтому 503, а не 500: по 5xx-семантике это «временно недоступно».
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Msg: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, secretResponse{Chat: s.cfg.Secret.Chat, Size: size})
}

// handleHealthz — liveness: процесс жив и способен отвечать.
// Здесь намеренно нет проверок зависимостей: иначе падение апстрима приведёт
// к перезапуску здорового пода.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// handleReadyz — readiness: готов ли сервис принимать трафик.
// В отличие от liveness, учитывает конфигурацию и снимается при shutdown.
func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	checks := map[string]string{}
	status := http.StatusOK

	if !s.ready.Load() {
		checks["shutdown"] = "in progress"
		status = http.StatusServiceUnavailable
	}
	if !s.cfg.Public.Configured() {
		checks["public"] = ErrPublicNotConfigured.Error()
		status = http.StatusServiceUnavailable
	}
	if s.upstream != nil && s.upstream.State() == upstream.StateOpen {
		checks["upstream"] = upstream.ErrCircuitOpen.Error()
		status = http.StatusServiceUnavailable
	}

	body := healthResponse{Status: "ready", Checks: checks}
	if status != http.StatusOK {
		body.Status = "not ready"
	}
	writeJSON(w, status, body)
}

// handleUpstream дёргает внешний сервис через клиента с бюджетом времени,
// ретраями и circuit breaker'ом.
func (s *Server) handleUpstream(w http.ResponseWriter, r *http.Request) {
	if s.upstream == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Msg: ErrUpstreamNotConfigured.Error()})
		return
	}

	// Контекст запроса протаскивается дальше: если клиент отвалился, нет
	// смысла продолжать ретраить в апстрим.
	res, err := s.upstream.Do(r.Context())
	switch {
	case errors.Is(err, upstream.ErrCircuitOpen):
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Msg: err.Error()})
		return
	case err != nil:
		writeJSON(w, http.StatusBadGateway, errorResponse{Msg: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, upstreamResponse{
		StatusCode: res.StatusCode,
		Attempts:   res.Attempts,
		ElapsedMs:  res.ElapsedMs,
	})
}

// hasSREHeader проверяет заголовок доступа. Сравнение регистронезависимое,
// как и было исторически: значение публичное, скрывать нечего.
func hasSREHeader(r *http.Request) bool {
	return equalFold(r.Header.Get(XIamSRE), xIamSREValue)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
