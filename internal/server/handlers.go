package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

func getRoot(w http.ResponseWriter, r *http.Request) {
	resp := prepareMsg(r)
	resp.msg = `{"msg":"This is home page"}`

	log.Info().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Msg("home page")

	n, err := io.WriteString(w, fmt.Sprintf("%s\n", resp.msg))
	if err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteSting err")
	}
	log.Debug().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Int("size", n).
		Msg("write bytes")
}

func getPing(w http.ResponseWriter, r *http.Request) {
	resp := prepareMsg(r)
	resp.msg = `{"msg":"pong"}`

	log.Info().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Msg("pong")

	n, err := io.WriteString(w, fmt.Sprintf("%s\n", resp.msg))
	if err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteSting err")
	}
	log.Debug().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Int("size", n).
		Msg("write bytes")
}

func getPublic(w http.ResponseWriter, r *http.Request) {
	resp := prepareMsg(r)
	messagePub := &msgPub{
		Discord: "https://discord.gg/aKZNvaXQmR",
		Chat:    "https://t.me/jtprogru_chat",
		Channel: "https://t.me/jtprogru_channel",
	}

	out, err := json.Marshal(messagePub)
	if err != nil {
		log.Error().AnErr("err", err).Msg("can't parse messagePub")
	}

	resp.msg = string(out)

	log.Info().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Msg("messagePub is parsed and marshaled")

	n, err := io.WriteString(w, fmt.Sprintf("%s\n", resp.msg))
	if err != nil {
		log.Error().AnErr("err", err).Msg("io.WriteSting err")
	}
	log.Debug().
		Str("server_addr", resp.server_addr).
		Str("remote_addr", resp.remote_addr).
		Str("user_agent", resp.user_agent).
		Str("uri", resp.uri).
		Int("size", n).
		Msg("write bytes")
}
