package server

import (
	"fmt"
	"io"
	"net/http"
)

type msg map[string]interface{}

func getRoot(w http.ResponseWriter, r *http.Request) {
	resp := make(msg)
	ctx := r.Context()

	resp["server_addr"] = ctx.Value(keyServerAddr)
	resp["user_agent"] = r.UserAgent()
	resp["remote_addr"] = r.RemoteAddr
	resp["uri"] = r.RequestURI
	resp["msg"] = "This is home page!"

	log.Infof("%s - %s - %s - %s\n", resp["server_addr"], resp["remote_addr"], resp["uri"], resp["msg"])

	io.WriteString(w, fmt.Sprintf("%s\n", resp["msg"]))
}

func getPing(w http.ResponseWriter, r *http.Request) {
	resp := make(msg)
	ctx := r.Context()

	resp["server_addr"] = ctx.Value(keyServerAddr)
	resp["user_agent"] = r.UserAgent()
	resp["remote_addr"] = r.RemoteAddr
	resp["uri"] = r.RequestURI
	resp["msg"] = "pong"

	log.Infof("%s - %s - %s - %s\n", resp["server_addr"], resp["remote_addr"], resp["uri"], resp["msg"])

	io.WriteString(w, fmt.Sprintf("%s\n", resp["msg"]))
}
