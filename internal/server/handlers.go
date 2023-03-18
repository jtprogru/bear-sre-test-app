package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

func getSecret(w http.ResponseWriter, r *http.Request) {
	resp := prepareMsg(r)

	xIamSreValue := strings.ToLower(r.Header.Get(XIamSRE))
	if xIamSreValue != "sre" {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := io.WriteString(w, fmt.Sprintf("%s\n", `{"msg":"access denied"}`))
		if err != nil {
			log.Error().AnErr("err", err).Msg("io.WriteSting err")
		}
		log.Error().
			Str("server_addr", resp.server_addr).
			Str("remote_addr", resp.remote_addr).
			Str("user_agent", resp.user_agent).
			Str("uri", resp.uri).
			AnErr("err", ErrHeaderXIamSRENotSet).
			Msg("header not set")
		return
	}

	size, err := checkSecretFile()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.msg = fmt.Sprintf(`{"msg":"%s"}`, err)
		_, err = io.WriteString(w, fmt.Sprintf("%s\n", resp.msg))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().
				Str("server_addr", resp.server_addr).
				Str("remote_addr", resp.remote_addr).
				Str("user_agent", resp.user_agent).
				Str("uri", resp.uri).
				AnErr("err", err).
				Msg("io.WriteSting err")
			return
		}
		log.Error().
			Str("server_addr", resp.server_addr).
			Str("remote_addr", resp.remote_addr).
			Str("user_agent", resp.user_agent).
			Str("uri", resp.uri).
			AnErr("err", err).
			Msg("check secret file err")
		return
	} else {
		messageSec := &msgSec{
			Chat: "https://t.me/+j7JspAH4gpxiMDVi",
			Size: size,
		}

		out, err := json.Marshal(messageSec)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().
				Str("server_addr", resp.server_addr).
				Str("remote_addr", resp.remote_addr).
				Str("user_agent", resp.user_agent).
				Str("uri", resp.uri).
				AnErr("err", err).
				Msg("can't parse messageSec")
		}
		resp.msg = string(out)
		n, err := io.WriteString(w, fmt.Sprintf("%s\n", resp.msg))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error().
				Str("server_addr", resp.server_addr).
				Str("remote_addr", resp.remote_addr).
				Str("user_agent", resp.user_agent).
				Str("uri", resp.uri).
				AnErr("err", err).
				Msg("io.WriteSting err")
			return
		}
		log.Debug().
			Str("server_addr", resp.server_addr).
			Str("remote_addr", resp.remote_addr).
			Str("user_agent", resp.user_agent).
			Str("uri", resp.uri).
			Int("size", n).
			Msg("secret message is sent")
		return
	}
}
