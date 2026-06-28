package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ hdlctrlv1connect.ControllerServiceHandler = (*ControllerService)(nil)

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
	souc           *usecase.ScheduledSessionOperationUsecase
	ajuc           *async_job.Usecase
	permUC         *usecase.PermissionUsecase
	groupRepo      port.GroupRepository
	roleRepo       port.RoleRepository
	skyfrostClient skyfrost.Client
	bus            notification.Bus
}

func NewControllerService(
	hhrepo port.HeadlessHostRepository,
	srepo port.SessionRepository,
	hhuc *usecase.HeadlessHostUsecase,
	hauc *usecase.HeadlessAccountUsecase,
	suc *usecase.SessionUsecase,
	buc *usecase.BlobUsecase,
	souc *usecase.ScheduledSessionOperationUsecase,
	ajuc *async_job.Usecase,
	permUC *usecase.PermissionUsecase,
	groupRepo port.GroupRepository,
	roleRepo port.RoleRepository,
	skyfrostClient skyfrost.Client,
	bus notification.Bus,
) *ControllerService {
	return &ControllerService{
		hhrepo:         hhrepo,
		srepo:          srepo,
		hhuc:           hhuc,
		hauc:           hauc,
		suc:            suc,
		buc:            buc,
		souc:           souc,
		ajuc:           ajuc,
		permUC:         permUC,
		groupRepo:      groupRepo,
		roleRepo:       roleRepo,
		skyfrostClient: skyfrostClient,
		bus:            bus,
	}
}

func (c *ControllerService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(
		logging.NewErrorLogInterceptor(),
		auth.NewAuthInterceptor(),
		NewPermissionInterceptor(c.permUC, PermissionDeps{
			HostRepo:    c.hhrepo,
			SessionRepo: c.srepo,
			AccountUC:   c.hauc,
			GroupRepo:   c.groupRepo,
			RoleRepo:    c.roleRepo,
		}),
	)

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

	if errors.Is(err, domain.ErrUnauthenticated) {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	if errors.Is(err, domain.ErrPermissionDenied) {
		return connect.NewError(connect.CodePermissionDenied, err)
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

// publishHostUpdated は単一ホストの再フェッチを促す通知を発行する.
// container 状態の変化は DockerEventWatcher 側でも publish されるので,
// ここでは「container 状態は変えないが UI 表示は更新したい」ケースで使う.
func (c *ControllerService) publishHostUpdated(hostID string) {
	c.bus.Publish(notification.HostUpdated(hostID, "", nil))
}

func (c *ControllerService) publishHostListChanged() {
	c.bus.Publish(notification.HostListChanged())
}

// resolveListGroupFilter は List 系 RPC 共通のグループフィルタ解決ヘルパー.
//
// requestedGroupID:
//   - nil または空文字: 自動絞り込み. system:group.list 保持なら nil (= 全件),
//     それ以外は user が permKey を持つグループ群に絞り込み.
//   - 非空: 指定グループに絞り込み. ただし user が当該グループの permKey または
//     system:group.list を持たない場合は PermissionDenied.
//
// 戻り値 groupIDs は repository / sqlc の sqlc.narg('group_ids') にそのまま渡せる:
//   - nil: 全件
//   - 空 slice: ゼロ件
//   - 非空 slice: ANY 絞り込み
//
// permission interceptor は List 系を requireAuthOnly で通すため、認可ロジックは
// この helper で集約する.
func (c *ControllerService) resolveListGroupFilter(ctx context.Context, requestedGroupID, permKey string) ([]string, error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if gid := strings.TrimSpace(requestedGroupID); gid != "" {
		ok, err := c.permUC.CanReadGroup(ctx, claims.UserID, gid, permKey)
		if err != nil {
			return nil, convertErr(err)
		}

		if !ok {
			return nil, connect.NewError(connect.CodePermissionDenied,
				errors.New("permission required: "+permKey))
		}

		return []string{gid}, nil
	}

	groupIDs, _, err := c.permUC.ResolveListGroupFilter(ctx, claims.UserID, permKey)
	if err != nil {
		return nil, convertErr(err)
	}
	// listAll == true なら groupIDs == nil で「全件」を伝える.
	// それ以外 (所属あり/なし) は groupIDs (空も含む) をそのまま返す.
	return groupIDs, nil
}
