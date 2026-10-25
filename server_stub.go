//go:build clientonly

package main

const DefaultBuildMode = "client"

type Server struct{}

func NewServer(port int, edge string, targetIP string) *Server {
	return &Server{}
}

func (s *Server) Start() error {
	return nil
}
