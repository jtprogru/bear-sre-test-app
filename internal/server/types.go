package server

import "errors"

type message struct {
	server_addr string
	remote_addr string
	user_agent  string
	uri         string
	msg         string
}

type str string

type msgPub struct {
	Discord string
	Chat    string
	Channel string
}

type msgSec struct {
	Chat string
	Size int64
}

const (
	keyServerAddr  str    = "serverAddr"
	secretFilePath string = "/tmp/jtprogru.test"
	secretFileSize int64  = 2048
	XIamSRE        string = "X-IAM-SRE"
)

var (
	ErrSecretFileNotFound   = errors.New("secret file not found")
	ErrSecretFileIsEmpty    = errors.New("secret file is empty")
	ErrSecretFileIsTooShort = errors.New("secret file is too short")
	ErrHeaderXIamSRENotSet  = errors.New("X-IAM-SRE header not set")
)
