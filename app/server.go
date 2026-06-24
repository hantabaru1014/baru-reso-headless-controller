package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	pkgerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/resonitelink"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/rpc"
	"github.com/hantabaru1014/baru-reso-headless-controller/front"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/blobstore"
	"github.com/hantabaru1014/baru-reso-headless-controller/worker"
)

type Server struct {
	userService        *rpc.UserService
	controllerService  *rpc.ControllerService
	imageChecker       *worker.ImageChecker
	eventWatcher       *worker.EventWatcher
	blobClient         blobstore.Client
	resoniteLinkBridge *resonitelink.Bridge
	httpServer         *http.Server
}

func NewServer(
	userService *rpc.UserService,
	controllerService *rpc.ControllerService,
	imageChecker *worker.ImageChecker,
	eventWatcher *worker.EventWatcher,
	blobClient blobstore.Client,
	resoniteLinkBridge *resonitelink.Bridge,
) *Server {
	return &Server{
		userService:        userService,
		controllerService:  controllerService,
		imageChecker:       imageChecker,
		eventWatcher:       eventWatcher,
		blobClient:         blobClient,
		resoniteLinkBridge: resoniteLinkBridge,
	}
}

// makeBlobHandler returns an HTTP handler that proxies blob downloads from
// the blob store. The UUID in the path acts as the capability token — there
// is no extra auth check, on the assumption that anyone with the UUID is
// authorized to download. Object lifecycle in the blob store enforces TTL.
func makeBlobHandler(blob blobstore.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uuidStr := mux.Vars(r)["uuid"]
		if _, err := uuid.Parse(uuidStr); err != nil {
			http.Error(w, "invalid uuid", http.StatusBadRequest)
			return
		}

		rc, length, contentType, filename, err := blob.GetObject(r.Context(), uuidStr)
		if err != nil {
			if errors.Is(err, blobstore.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			slog.Error("failed to fetch blob", "error", err, "uuid", uuidStr)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		defer func() { _ = rc.Close() }()

		if contentType == "" {
			contentType = "application/octet-stream"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))

		if filename != "" {
			w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(filename))
		} else {
			w.Header().Set("Content-Disposition", "attachment")
		}

		w.Header().Set("Cache-Control", "no-store")

		if r.Method == http.MethodHead {
			return
		}

		if _, err := io.Copy(w, rc); err != nil {
			slog.Warn("failed to stream blob to client", "error", err, "uuid", uuidStr)
		}
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
		_ = f.Close()

		http.ServeFileFS(w, r, front.FrontAssets, filePath)
	} else {
		http.ServeFileFS(w, r, front.FrontAssets, "index.html")
	}
}

func (s *Server) ListenAndServe(addr string, frontUrl string) error {
	bucketCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) //nolint:mnd // startup
	if err := s.blobClient.EnsureBucket(bucketCtx); err != nil {
		cancel()
		return pkgerrors.Wrap(err, 0)
	}

	cancel()

	s.imageChecker.Start()
	s.eventWatcher.Start()

	router := mux.NewRouter()

	{
		p, h := s.userService.NewHandler()
		router.PathPrefix(p).Handler(h)
	}
	{
		p, h := s.controllerService.NewHandler()
		router.PathPrefix(p).Handler(h)
	}

	router.HandleFunc("/blobs/{uuid}", makeBlobHandler(s.blobClient)).Methods(http.MethodGet, http.MethodHead)
	router.HandleFunc(resonitelink.WSPath, s.resoniteLinkBridge.ServeHTTP).Methods(http.MethodGet)

	if len(frontUrl) > 0 {
		rpURL, err := url.Parse(frontUrl)
		if err != nil {
			return pkgerrors.Wrap(err, 0)
		}

		proxy := httputil.NewSingleHostReverseProxy(rpURL)
		router.NotFoundHandler = proxy
	} else {
		router.NotFoundHandler = http.HandlerFunc(spaFileHandler)
	}

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           router,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second, //nolint:mnd // standard timeout
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown は現在進行中のリクエストを完了させてからサーバーを停止する.
func (s *Server) Shutdown(ctx context.Context) error {
	s.imageChecker.Stop()
	s.eventWatcher.Stop()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}
