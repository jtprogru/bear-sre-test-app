package server

import (
	"net/http"
	"os"
)

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

func checkSecretFile() (int64, error) {
	f, err := os.Open(secretFilePath)
	if err != nil {
		return -1, ErrSecretFileNotFound
	}

	fStat, err := f.Stat()
	if err != nil {
		return -1, ErrSecretFileNotFound
	}

	if fStat.Size() < secretFileSize {
		return -1, ErrSecretFileIsTooShort
	}

	return fStat.Size(), nil
}
