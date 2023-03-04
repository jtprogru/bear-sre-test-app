package config

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type cfg struct {
	addr            string
	chanelBirthDate string
	chatBirthDate   string
	log             *logrus.Logger
}

func New() *cfg {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.testapp")
	viper.AddConfigPath("/etc/testapp")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("can't load config")
	}

	port := viper.GetInt("prod.port")
	chanelBirthDate := viper.GetString("tg.chanelBirthDate")
	chatBirthDate := viper.GetString("tg.chatBirthDate")
	return &cfg{
		addr:            fmt.Sprintf(":%d", port),
		chanelBirthDate: chanelBirthDate,
		chatBirthDate:   chatBirthDate,
		log:             log,
	}
}
func (c *cfg) Addr() string {
	c.log.Infof("get addr: %v", c.addr)
	return c.addr
}

func (c *cfg) ChanelBD() string {
	c.log.Infof("get chanelBirthDate: %v", c.chanelBirthDate)
	return c.chanelBirthDate
}

func (c *cfg) ChatBD() string {
	c.log.Infof("get chatBirthDate: %v", c.chatBirthDate)
	return c.chatBirthDate
}
