package server

import "net/http"

func prepareMsg(r *http.Request) *message {
	resp := &message{}
	ctx := r.Context()
	srv_addr := ctx.Value(keyServerAddr)

	resp.server_addr = srv_addr.(string)
	resp.user_agent = r.UserAgent()
	resp.remote_addr = r.RemoteAddr
	resp.uri = r.RequestURI
	return resp

}
