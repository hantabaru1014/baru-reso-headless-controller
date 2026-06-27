package rpc

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/notification"
)

var _ hdlctrlv1connect.NotificationServiceHandler = (*NotificationService)(nil)

// keepAliveInterval は HTTP/1.1 chunked transfer や中間 proxy のアイドル
// タイムアウト対策で送る KeepAlive 間隔. nginx の proxy_read_timeout
// デフォルト 60s を下回るように余裕を持って 30s に設定.
const keepAliveInterval = 30 * time.Second

type NotificationService struct {
	bus notification.Bus
}

func NewNotificationService(bus notification.Bus) *NotificationService {
	return &NotificationService{bus: bus}
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
// を server-streaming で push する. 30 秒ごとに KeepAlive を送って proxy の
// アイドル切断を防ぐ. context.Done() か stream.Send 失敗で抜けて bus
// subscriber を解放する.
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

			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}
