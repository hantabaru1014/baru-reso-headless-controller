package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-errors/errors"
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
	httpServer        *http.Server
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

func spaFileHandler(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path[1:] // Remove leading "/"
	slog.Debug("Serve http", "path", filePath)

	if filePath == "" {
		filePath = "index.html"
	}
	f, err := front.FrontAssets.Open(filePath)
	if err == nil {
		f.Close()
		http.ServeFileFS(w, r, front.FrontAssets, filePath)
	} else {
		http.ServeFileFS(w, r, front.FrontAssets, "index.html")
	}
}

func (s *Server) ListenAndServe(addr string, frontUrl string) error {
	s.imageChecker.Start()

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
			return errors.Wrap(err, 0)
		}
		proxy := httputil.NewSingleHostReverseProxy(rpURL)
		router.NotFoundHandler = proxy
	} else {
		router.NotFoundHandler = http.HandlerFunc(spaFileHandler)
	}

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(router, &http2.Server{}),
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown は現在進行中のリクエストを完了させてからサーバーを停止する
func (s *Server) Shutdown(ctx context.Context) error {
	s.imageChecker.Stop()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
