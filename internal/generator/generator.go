package generator

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

type gen struct{}

func New() *gen {
	return &gen{}
}

func (g *gen) Gen() string {
	h, _ := os.Hostname()
	return base64.StdEncoding.EncodeToString(
		[]byte(
			fmt.Sprintf(
				"%s.%d-%d-%dT%d:%d:%d\n",
				h,
				2015, 11, 10, 0, 0, 0,
			),
		),
	)
}

func (g *gen) Hostname() string {
	h, _ := os.Hostname()
	return h
}

func (g *gen) TS() int64 {
	return time.Now().UnixMicro()
}

func (g *gen) Fuck() any {

	for _, v := range os.Environ() {
		s := strings.Split(v, "=")[0]
		if s == "SRV_SRE_FLAG" {
			return os.Getenv(s)
		}
	}
	return "FUCK"
}
