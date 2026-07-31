package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	msg := `{"msg":"This is home page"}`

	if _, err := io.WriteString(w, fmt.Sprintf("%s\n", msg)); err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteString err")
	}
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) {
	msg := `{"msg":"pong"}`

	if _, err := io.WriteString(w, fmt.Sprintf("%s\n", msg)); err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteString err")
	}
}

func (s *Server) handlePublic(w http.ResponseWriter, _ *http.Request) {
	if !s.cfg.Public.Configured() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := io.WriteString(w, fmt.Sprintf(`{"msg":"%s"}`, ErrPublicNotConfigured))
		if err != nil {
			log.Error().AnErr("err", err).Msg("io.WriteString err")
		}
		return
	}

	out, err := json.Marshal(publicResponse{
		Discord: s.cfg.Public.Discord,
		Chat:    s.cfg.Public.Chat,
		Channel: s.cfg.Public.Channel,
	})
	if err != nil {
		log.Error().AnErr("err", err).Msg("can't marshal public links")
	}

	if _, err := io.WriteString(w, fmt.Sprintf("%s\n", out)); err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteString err")
	}
}

func (s *Server) handleSecret(w http.ResponseWriter, r *http.Request) {
	if !hasSREHeader(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := io.WriteString(w, fmt.Sprintf(`{"msg":"%s"}`, ErrHeaderXIamSRENotSet))
		if err != nil {
			log.Error().AnErr("err", err).Msg("io.WriteString err")
		}
		return
	}

	size, err := checkSecretFile(s.cfg.Secret.FilePath, s.cfg.Secret.MinSize)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err = io.WriteString(w, fmt.Sprintf(`{"msg":"%s"}`, err))
		if err != nil {
			log.Error().AnErr("err", err).Msg("io.WriteString err")
		}
		return
	}

	out, err := json.Marshal(secretResponse{Chat: s.cfg.Secret.Chat, Size: size})
	if err != nil {
		log.Error().AnErr("err", err).Msg("can't marshal secret response")
	}

	if _, err := io.WriteString(w, fmt.Sprintf("%s\n", out)); err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteString err")
	}
}

// hasSREHeader проверяет заголовок доступа. Сравнение регистронезависимое.
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
