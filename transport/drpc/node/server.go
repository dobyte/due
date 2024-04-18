package node

import "github.com/dobyte/due/v2/transport/drpc/internal/server"

func NewServer() {
	s, err := server.NewServer()

	s.RegisterHandler()
}
