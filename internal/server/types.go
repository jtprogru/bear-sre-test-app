package server

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

const (
	keyServerAddr str = "serverAddr"
)
