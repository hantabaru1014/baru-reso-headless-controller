package app

import (
	"net/http"

	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	userService       *rpc.UserService
	controllerService *rpc.ControllerService
}

func NewServer(
	userService *rpc.UserService,
	controllerService *rpc.ControllerService,
) *Server {
	return &Server{
		userService:       userService,
		controllerService: controllerService,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	s.userService.Handle(mux)
	s.controllerService.Handle(mux)

	return http.ListenAndServe(
		addr,
		h2c.NewHandler(mux, &http2.Server{}),
	)
}
