package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

type config struct {
	addr            string `env:"SRV_ADDR"`
	chanelBirthDate time.Time
	chatBirthDate   time.Time
}

func New() *config {
	addr := os.Getenv("SRV_ADDR")
	if addr == "" {
		log.Fatalln("Хьюстон! У нас проблемы!")
	}
	return &config{
		addr:            fmt.Sprintf(":%s", addr),
		chanelBirthDate: time.Date(2015, 11, 10, 0, 0, 0, 0, time.FixedZone("MSK", 3*60*60)),
		chatBirthDate:   time.Date(2019, 06, 01, 0, 0, 0, 0, time.FixedZone("MSK", 3*60*60)),
	}
}

func (c *config) Addr() string {
	return c.addr
}
