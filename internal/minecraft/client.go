package minecraft

import "net"

type Client struct {
	Conn          net.Conn
	ServerAddress string
	Protocol      int
	Username      string
}
