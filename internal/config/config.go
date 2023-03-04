package config

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type cfg struct {
	addr            string
	chanelBirthDate string
	chatBirthDate   string
}

func New() *cfg {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.testapp")
	viper.AddConfigPath("/etc/testapp")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal().Msg("can't load config")
	}

	port := viper.GetInt("prod.port")
	chanelBirthDate := viper.GetString("tg.chanelBirthDate")
	chatBirthDate := viper.GetString("tg.chatBirthDate")
	return &cfg{
		addr:            fmt.Sprintf(":%d", port),
		chanelBirthDate: chanelBirthDate,
		chatBirthDate:   chatBirthDate,
	}
}
func (c *cfg) Addr() string {
	log.Info().Msg(fmt.Sprintf("get addr: %v", c.addr))
	return c.addr
}

func (c *cfg) ChanelBD() string {
	log.Info().Msg(fmt.Sprintf("get chanelBirthDate: %v", c.chanelBirthDate))
	return c.chanelBirthDate
}

func (c *cfg) ChatBD() string {
	log.Info().Msg(fmt.Sprintf("get chatBirthDate: %v", c.chatBirthDate))
	return c.chatBirthDate
}
