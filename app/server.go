package app

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/front"
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

func (s *Server) ListenAndServe(addr string, frontUrl string) error {
	router := mux.NewRouter()

	{
		p, h := s.userService.NewHandler()
		router.PathPrefix(p).Handler(h)
	}
	{
		p, h := s.controllerService.NewHandler()
		router.PathPrefix(p).Handler(h)
	}

	if len(frontUrl) > 0 {
		rpURL, err := url.Parse(frontUrl)
		if err != nil {
			return err
		}
		proxy := httputil.NewSingleHostReverseProxy(rpURL)
		router.NotFoundHandler = proxy
	} else {
		router.NotFoundHandler = http.FileServerFS(front.FrontAssets)
	}

	return http.ListenAndServe(
		addr,
		h2c.NewHandler(router, &http2.Server{}),
	)
}
