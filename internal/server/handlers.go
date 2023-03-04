package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jtprogru/bear-sre-test-app/internal/generator"
)

type msg map[string]interface{}

func getRoot(w http.ResponseWriter, r *http.Request) {
	resp := make(msg)
	ctx := r.Context()
	gen := generator.New()

	resp["server_addr"] = ctx.Value(keyServerAddr)
	resp["user_agent"] = r.UserAgent()
	resp["remote_addr"] = r.RemoteAddr
	resp["uri"] = r.RequestURI
	resp["msg"] = "This is home page!"
	resp["fuck"] = gen.Fuck()

	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s\n", string(b))

	io.WriteString(w, fmt.Sprintf("%s\n", string(b)))
}

func getPing(w http.ResponseWriter, r *http.Request) {
	resp := make(msg)
	ctx := r.Context()
	gen := generator.New()

	resp["server_addr"] = ctx.Value(keyServerAddr)
	resp["user_agent"] = r.UserAgent()
	resp["remote_addr"] = r.RemoteAddr
	resp["uri"] = r.RequestURI
	resp["msg"] = "pong"
	resp["ts"] = gen.TS()
	resp["code"] = gen.Gen()
	resp["hostname"] = gen.Hostname()
	resp["fuck"] = gen.Fuck()

	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s\n", string(b))

	io.WriteString(w, fmt.Sprintf("%s\n", string(b)))
}
