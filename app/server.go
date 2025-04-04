package app

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/front"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	userService       *rpc.UserService
	controllerService *rpc.ControllerService
	imageChecker      *worker.ImageChecker
}

func NewServer(
	userService *rpc.UserService,
	controllerService *rpc.ControllerService,
	imageChecker *worker.ImageChecker,
) *Server {
	return &Server{
		userService:       userService,
		controllerService: controllerService,
		imageChecker:      imageChecker,
	}
}

func (s *Server) ListenAndServe(addr string, frontUrl string) error {
	s.imageChecker.Start()
	defer s.imageChecker.Stop()

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
