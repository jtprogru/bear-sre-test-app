package server

import "errors"

// ctxKey — собственный тип для ключей контекста, чтобы исключить коллизии
// с ключами других пакетов.
type ctxKey string

const (
	keyServerAddr ctxKey = "serverAddr"

	// XIamSRE — заголовок доступа к /secret.
	XIamSRE = "X-IAM-SRE"
	// xIamSREValue — ожидаемое значение заголовка (сравнение регистронезависимо).
	xIamSREValue = "sre"
)

var (
	ErrSecretFileNotFound   = errors.New("secret file not found")
	ErrSecretFileIsEmpty    = errors.New("secret file is empty")
	ErrSecretFileIsTooShort = errors.New("secret file is too short")
	ErrHeaderXIamSRENotSet  = errors.New("X-IAM-SRE header not set")
	ErrPublicNotConfigured  = errors.New("public links are not configured")
	ErrNotFound             = errors.New("not found")
	ErrMethodNotAllowed     = errors.New("method not allowed")
)

// errorResponse — единый формат ошибки для всех ручек.
type errorResponse struct {
	Msg string `json:"msg"`
}

// msgResponse — простой текстовый ответ.
type msgResponse struct {
	Msg string `json:"msg"`
}

// publicResponse — тело ответа /public.
type publicResponse struct {
	Discord string `json:"discord"`
	Chat    string `json:"chat"`
	Channel string `json:"channel"`
}

// secretResponse — тело ответа /secret.
type secretResponse struct {
	Chat string `json:"chat"`
	Size int64  `json:"size"`
}

// healthResponse — тело ответа /healthz и /readyz.
type healthResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}
