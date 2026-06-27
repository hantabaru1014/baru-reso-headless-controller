package rpc

import (
	"errors"
	"net/http"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ hdlctrlv1connect.ControllerServiceHandler = (*ControllerService)(nil)

const defaultRestartTimeoutSeconds = 10 * 60

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// normalizePageRequest はクライアントから渡された PageRequest をサーバー側の規約に従って正規化する。
// - page_index < 0 は CodeInvalidArgument
// - page_size <= 0 はデフォルト 20
// - page_size > 100 は 100 にクランプ
// 返り値の pageIndex / pageSize はそのまま usecase に渡し、PageResponse にも詰めて返す。
func normalizePageRequest(p *hdlctrlv1.PageRequest) (int32, int32, error) {
	if p == nil {
		return 0, defaultPageSize, nil
	}

	if p.GetPageIndex() < 0 {
		return 0, 0, connect.NewError(connect.CodeInvalidArgument, errors.New("page_index must be >= 0"))
	}

	pageSize := p.GetPageSize()
	switch {
	case pageSize <= 0:
		pageSize = defaultPageSize
	case pageSize > maxPageSize:
		pageSize = maxPageSize
	}

	return p.GetPageIndex(), pageSize, nil
}

type ControllerService struct {
	hhrepo         port.HeadlessHostRepository
	srepo          port.SessionRepository
	hhuc           *usecase.HeadlessHostUsecase
	hauc           *usecase.HeadlessAccountUsecase
	suc            *usecase.SessionUsecase
	buc            *usecase.BlobUsecase
	skyfrostClient skyfrost.Client
}

func NewControllerService(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, hhuc *usecase.HeadlessHostUsecase, hauc *usecase.HeadlessAccountUsecase, suc *usecase.SessionUsecase, buc *usecase.BlobUsecase, skyfrostClient skyfrost.Client) *ControllerService {
	return &ControllerService{
		hhrepo:         hhrepo,
		srepo:          srepo,
		hhuc:           hhuc,
		hauc:           hauc,
		suc:            suc,
		buc:            buc,
		skyfrostClient: skyfrostClient,
	}
}

func (c *ControllerService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(logging.NewErrorLogInterceptor(), auth.NewAuthInterceptor())

	return hdlctrlv1connect.NewControllerServiceHandler(c, interceptors)
}

// convertErr converts domain errors to appropriate Connect RPC error codes.
func convertErr(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, domain.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}

	// ErrHostDraining is a precondition violation — the host has been
	// enrolled for an auto-upgrade and is no longer accepting new
	// sessions. Surface the distinction so the frontend can show a
	// proper "host is upgrading, please retry shortly" message instead
	// of a generic internal error.
	if errors.Is(err, usecase.ErrHostDraining) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}

	return connect.NewError(connect.CodeInternal, err)
}

// convertRpcClientErr converts domain errors to appropriate Connect RPC error codes for RPC client operations.
func convertRpcClientErr(err error) error {
	if err == nil {
		return nil
	}

	return connect.NewError(connect.CodeInternal, err)
}
