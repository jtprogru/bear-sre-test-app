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

func checkSecretFile() error {
	f, err := os.Open(secretFilePath)
	if err != nil {
		return ErrSecretFileNotFound
	}

	fStat, err := f.Stat()
	if err != nil {
		return ErrSecretFileNotFound
	}

	if fStat.Size() < secretFileSize {
		return ErrSecretFileIsTooShort
	}

    return nil
}
