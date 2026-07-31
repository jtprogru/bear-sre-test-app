// Package config загружает настройки приложения из YAML-файла.
package config

import (
	"errors"
	"fmt"
	"os"
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
	Discord string `json:"discord"`
	Chat    string `json:"chat"`
	Channel string `json:"channel"`
}

// Configured сообщает, заполнены ли ссылки. Ручка /public отдаёт 503, пока
// конфигурация неполна, — это и есть «правильно выставленный параметр» из README.
func (p PublicLinks) Configured() bool {
	return p.Discord != "" && p.Chat != "" && p.Channel != ""
}

// SecretConfig — параметры ручки /secret.
type SecretConfig struct {
	Chat     string
	FilePath string
	MinSize  int64
}

// Config — итоговая конфигурация приложения.
type Config struct {
	Addr            string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration

	ChannelBirthDate string
	ChatBirthDate    string

	Public PublicLinks
	Secret SecretConfig
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("prod.port", 8080)
	v.SetDefault("server.shutdownTimeout", 10*time.Second)
	v.SetDefault("server.readTimeout", 10*time.Second)
	v.SetDefault("server.writeTimeout", 10*time.Second)
	v.SetDefault("server.idleTimeout", 60*time.Second)
	v.SetDefault("secret.filePath", "/tmp/jtprogru.test")
	v.SetDefault("secret.minSize", 2048)
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

	if err := v.ReadInConfig(); err != nil {
		return nil, ErrConfigNotFound
	}

	port := v.GetInt("prod.port")

	return &Config{
		Addr:            fmt.Sprintf(":%d", port),
		ShutdownTimeout: v.GetDuration("server.shutdownTimeout"),
		ReadTimeout:     v.GetDuration("server.readTimeout"),
		WriteTimeout:    v.GetDuration("server.writeTimeout"),
		IdleTimeout:     v.GetDuration("server.idleTimeout"),

		ChannelBirthDate: v.GetString("tg.chanelBirthDate"),
		ChatBirthDate:    v.GetString("tg.chatBirthDate"),

		Public: PublicLinks{
			Discord: v.GetString("public.discord"),
			Chat:    v.GetString("public.chat"),
			Channel: v.GetString("public.channel"),
		},
		Secret: SecretConfig{
			Chat:     v.GetString("secret.chat"),
			FilePath: v.GetString("secret.filePath"),
			MinSize:  v.GetInt64("secret.minSize"),
		},
	}, nil
}
