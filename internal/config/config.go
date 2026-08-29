// Package config загружает настройки приложения из YAML-файла и окружения.
//
// Приоритет источников (по убыванию):
//  1. переменные окружения с префиксом TESTAPP_ (например TESTAPP_PROD_PORT);
//  2. legacy-переменная SRV_ADDR — только порт, для совместимости с .env;
//  3. значения из config.yaml;
//  4. дефолты, заданные в setDefaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ErrConfigNotFound возвращается, если config.yaml не найден ни в одном из
// каталогов поиска. Отделён от ошибок разбора, чтобы вызывающий код мог
// различить «файла нет» и «файл битый».
var ErrConfigNotFound = errors.New("config file not found")

// searchPaths — каталоги поиска config.yaml в порядке приоритета.
var searchPaths = []string{".", "$HOME/.testapp", "/etc/testapp"}

// SearchPaths возвращает раскрытый список каталогов, в которых искался конфиг.
// Нужен для диагностики: без него сообщение «can't load config» бесполезно.
func SearchPaths() []string {
	out := make([]string, 0, len(searchPaths))
	for _, p := range searchPaths {
		out = append(out, os.ExpandEnv(p))
	}
	return out
}

// PublicLinks — ссылки, отдаваемые ручкой /public.
type PublicLinks struct {
	Chat    string `json:"chat"`
	Channel string `json:"channel"`
}

// Configured сообщает, заполнены ли ссылки. Ручка /public отдаёт 503, пока
// конфигурация неполна, — это и есть «правильно выставленный параметр» из README.
func (p PublicLinks) Configured() bool {
	return p.Chat != "" && p.Channel != ""
}

// SecretConfig — параметры ручки /secret.
type SecretConfig struct {
	Chat     string
	FilePath string
	MinSize  int64
}

// UpstreamConfig — параметры обращения к внешнему сервису.
type UpstreamConfig struct {
	URL              string
	Timeout          time.Duration
	MaxAttempts      int
	BackoffBase      time.Duration
	FailureThreshold int
	OpenFor          time.Duration
}

// Enabled сообщает, настроен ли апстрим. Если нет — ручка /upstream отдаёт 503.
func (u UpstreamConfig) Enabled() bool { return u.URL != "" }

// Config — итоговая конфигурация приложения.
type Config struct {
	Addr            string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration

	ChannelBirthDate string
	ChatBirthDate    string

	Public   PublicLinks
	Secret   SecretConfig
	Upstream UpstreamConfig
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("prod.port", 8080)
	v.SetDefault("server.shutdownTimeout", 10*time.Second)
	v.SetDefault("server.readTimeout", 10*time.Second)
	v.SetDefault("server.writeTimeout", 10*time.Second)
	v.SetDefault("server.idleTimeout", 60*time.Second)
	v.SetDefault("secret.filePath", "/tmp/jtprogru.test")
	v.SetDefault("secret.minSize", 2048)
	v.SetDefault("upstream.timeout", 2*time.Second)
	v.SetDefault("upstream.maxAttempts", 3)
	v.SetDefault("upstream.backoffBase", 50*time.Millisecond)
	v.SetDefault("upstream.failureThreshold", 5)
	v.SetDefault("upstream.openFor", 5*time.Second)
}

// New читает конфигурацию. Ошибку возвращает, а не гасит процесс: решение о
// том, фатально это или нет, принимает вызывающий код.
func New() (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, p := range searchPaths {
		v.AddConfigPath(p)
	}

	v.SetEnvPrefix("TESTAPP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		// Файла нет — это допустимо ровно в одном случае: порт задан через
		// окружение. Иначе честно говорим, где именно искали.
		if !portInEnv() {
			return nil, fmt.Errorf("%w in %s", ErrConfigNotFound, strings.Join(SearchPaths(), ", "))
		}
	}

	port := v.GetInt("prod.port")
	if legacy, ok := legacyPort(); ok {
		port = legacy
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid prod.port: %d (want 1..65535)", port)
	}

	return &Config{
		Addr:            fmt.Sprintf(":%d", port),
		ShutdownTimeout: v.GetDuration("server.shutdownTimeout"),
		ReadTimeout:     v.GetDuration("server.readTimeout"),
		WriteTimeout:    v.GetDuration("server.writeTimeout"),
		IdleTimeout:     v.GetDuration("server.idleTimeout"),

		ChannelBirthDate: v.GetString("tg.chanelBirthDate"),
		ChatBirthDate:    v.GetString("tg.chatBirthDate"),

		Public: PublicLinks{
			Chat:    v.GetString("public.chat"),
			Channel: v.GetString("public.channel"),
		},
		Secret: SecretConfig{
			Chat:     v.GetString("secret.chat"),
			FilePath: v.GetString("secret.filePath"),
			MinSize:  v.GetInt64("secret.minSize"),
		},
		Upstream: UpstreamConfig{
			URL:              v.GetString("upstream.url"),
			Timeout:          v.GetDuration("upstream.timeout"),
			MaxAttempts:      v.GetInt("upstream.maxAttempts"),
			BackoffBase:      v.GetDuration("upstream.backoffBase"),
			FailureThreshold: v.GetInt("upstream.failureThreshold"),
			OpenFor:          v.GetDuration("upstream.openFor"),
		},
	}, nil
}

// portInEnv сообщает, задан ли порт через окружение.
func portInEnv() bool {
	if _, ok := legacyPort(); ok {
		return true
	}
	return strings.TrimSpace(os.Getenv("TESTAPP_PROD_PORT")) != ""
}

// legacyPort читает SRV_ADDR из окружения. Историческая переменная из .env:
// раньше она объявлялась в трёх местах, но кодом не использовалась нигде.
func legacyPort() (int, bool) {
	raw := strings.TrimSpace(os.Getenv("SRV_ADDR"))
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(raw, ":"))
	if err != nil {
		return 0, false
	}
	return n, true
}
