package rpc

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ hdlctrlv1connect.NotificationServiceHandler = (*NotificationService)(nil)

// keepAliveInterval は HTTP/1.1 chunked transfer や中間 proxy のアイドル
// タイムアウト対策で送る KeepAlive 間隔. nginx の proxy_read_timeout
// デフォルト 60s を下回るように余裕を持って 30s に設定.
const keepAliveInterval = 30 * time.Second

// notificationPermCacheTTL は subscriber ごとの配信可否キャッシュの有効期間.
// イベントごとの DB 参照を抑えつつ、権限剥奪がこの時間内に配信へ反映される.
const notificationPermCacheTTL = 30 * time.Second

type NotificationService struct {
	bus      notification.Bus
	hostRepo port.HeadlessHostRepository
	permUC   *usecase.PermissionUsecase
}

func NewNotificationService(bus notification.Bus, hostRepo port.HeadlessHostRepository, permUC *usecase.PermissionUsecase) *NotificationService {
	return &NotificationService{bus: bus, hostRepo: hostRepo, permUC: permUC}
}

func (s *NotificationService) NewHandler() (string, http.Handler) {
	return hdlctrlv1connect.NewNotificationServiceHandler(
		s,
		connect.WithInterceptors(
			logging.NewErrorLogInterceptor(),
			auth.NewAuthInterceptor(),
		),
	)
}

// SubscribeNotifications は認証済みクライアントに対して NotificationEvent
// を server-streaming で push する. bus は全 subscriber へブロードキャスト
// するため、送信前にイベントの host が属するグループへの閲覧権限で絞り込む
// (グループ分離: 他グループのホスト/セッション活動を漏らさない).
// 30 秒ごとに KeepAlive を送って proxy のアイドル切断を防ぐ.
// context.Done() か stream.Send 失敗で抜けて bus subscriber を解放する.
func (s *NotificationService) SubscribeNotifications(
	ctx context.Context,
	_ *connect.Request[hdlctrlv1.SubscribeNotificationsRequest],
	stream *connect.ServerStream[hdlctrlv1.NotificationEvent],
) error {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	ch, cancel := s.bus.Subscribe(ctx, claims.UserID)
	defer cancel()

	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	permCache := map[string]notificationPermCacheEntry{}

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if err := stream.Send(notification.KeepAlive()); err != nil {
				return err
			}

		case ev, ok := <-ch:
			if !ok {
				return nil
			}

			if !s.canReceiveEvent(ctx, claims.UserID, ev, permCache) {
				continue
			}

			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}

type notificationPermCacheEntry struct {
	allowed   bool
	expiresAt time.Time
}

// canReceiveEvent は subscriber (userID) が ev を受信してよいか判定する.
// host に紐付くイベントは host の group への閲覧権限を要求し、判定結果を
// notificationPermCacheTTL の間キャッシュする.
//
// 権限解決に失敗した場合は fail-closed で「この 1 件は」配信しないが、その否定
// 結果はキャッシュしない. 一時的な DB 障害を 30 秒間の配信停止に固定しないため
// (次のイベントで再試行される).
func (s *NotificationService) canReceiveEvent(
	ctx context.Context,
	userID string,
	ev *hdlctrlv1.NotificationEvent,
	cache map[string]notificationPermCacheEntry,
) bool {
	hostID, permKeys, gated := notificationEventScope(ev)
	if !gated {
		return true
	}

	// gated だが host_id が無い = 未知/認可単位を解決できないイベント → fail-closed.
	if hostID == "" {
		return false
	}

	cacheKey := hostID + "\x00" + permKeys[0]
	if e, ok := cache[cacheKey]; ok && time.Now().Before(e.expiresAt) {
		return e.allowed
	}

	groupID, err := s.hostRepo.GetGroupID(ctx, hostID)
	if err != nil {
		slog.WarnContext(ctx, "notification: failed to resolve host group; dropping this event only",
			"hostID", hostID, "userID", userID, "err", err)

		return false
	}

	allowed, err := s.permUC.CanReadGroupAny(ctx, userID, groupID, permKeys)
	if err != nil {
		slog.WarnContext(ctx, "notification: permission check failed; dropping this event only",
			"hostID", hostID, "groupID", groupID, "userID", userID, "err", err)

		return false
	}

	cache[cacheKey] = notificationPermCacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().Add(notificationPermCacheTTL),
	}

	return allowed
}

// notificationEventScope はイベントの認可単位 (host_id, 受信に必要な permission key
// 群 (OR), 認可要否) を返す.
//
// 認可を要さない (gated=false) のは、リソース識別子を含まない KeepAlive /
// HostListChanged と、PublishTo で宛先を絞って配信される JobCompleted のみを
// 明示列挙する. それ以外の未知 payload は fail-closed (gated=true, host_id 空 =
// 配信不可) とし、将来 host/session scoped イベントを追加した際に本 switch の
// 更新を忘れても全ユーザーへ漏れないようにする.
//
// permission key は write-implies-read: host:write / session:write 保持者も
// 対応する read イベントを受信できる (usecase 層 requireHostRead と揃える).
func notificationEventScope(ev *hdlctrlv1.NotificationEvent) (string, []string, bool) {
	switch p := ev.GetPayload().(type) {
	case *hdlctrlv1.NotificationEvent_HostUpdated:
		return p.HostUpdated.GetHostId(), []string{entity.PermKey_HostRead, entity.PermKey_HostWrite}, true
	case *hdlctrlv1.NotificationEvent_SessionUpdated:
		return p.SessionUpdated.GetHostId(), []string{entity.PermKey_SessionRead, entity.PermKey_SessionWrite}, true
	case *hdlctrlv1.NotificationEvent_SessionUserChanged:
		return p.SessionUserChanged.GetHostId(), []string{entity.PermKey_SessionRead, entity.PermKey_SessionWrite}, true
	case *hdlctrlv1.NotificationEvent_SessionLifecycle:
		return p.SessionLifecycle.GetHostId(), []string{entity.PermKey_SessionRead, entity.PermKey_SessionWrite}, true
	case *hdlctrlv1.NotificationEvent_KeepAlive,
		*hdlctrlv1.NotificationEvent_HostListChanged,
		*hdlctrlv1.NotificationEvent_JobCompleted:
		// broadcast-safe (識別子を持たない / PublishTo で宛先限定済み).
		return "", nil, false
	default:
		// 未知 payload は fail-closed.
		return "", nil, true
	}
}
