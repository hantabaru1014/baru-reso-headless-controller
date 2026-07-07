package rpc

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ hdlctrlv1connect.MessageServiceHandler = (*MessageService)(nil)

type MessageService struct {
	muc    *usecase.MessageUsecase
	permUC *usecase.PermissionUsecase

	// permission interceptor 依存
	hostRepo    port.HeadlessHostRepository
	sessionRepo port.SessionRepository
	hauc        *usecase.HeadlessAccountUsecase
	groupRepo   port.GroupRepository
	roleRepo    port.RoleRepository
}

func NewMessageService(
	muc *usecase.MessageUsecase,
	permUC *usecase.PermissionUsecase,
	hostRepo port.HeadlessHostRepository,
	sessionRepo port.SessionRepository,
	hauc *usecase.HeadlessAccountUsecase,
	groupRepo port.GroupRepository,
	roleRepo port.RoleRepository,
) *MessageService {
	return &MessageService{
		muc:         muc,
		permUC:      permUC,
		hostRepo:    hostRepo,
		sessionRepo: sessionRepo,
		hauc:        hauc,
		groupRepo:   groupRepo,
		roleRepo:    roleRepo,
	}
}

func (s *MessageService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(
		logging.NewErrorLogInterceptor(),
		auth.NewAuthInterceptor(),
		NewPermissionInterceptor(s.permUC, PermissionDeps{
			HostRepo:    s.hostRepo,
			SessionRepo: s.sessionRepo,
			AccountUC:   s.hauc,
			GroupRepo:   s.groupRepo,
			RoleRepo:    s.roleRepo,
		}),
	)

	return hdlctrlv1connect.NewMessageServiceHandler(s, interceptors)
}

// ListMessages: 認証のみ (閲覧可能な範囲は usecase 側で絞り込む).
var _ = registerRPCPermission(
	hdlctrlv1connect.MessageServiceListMessagesProcedure,
	requireAuthenticated,
)

func (s *MessageService) ListMessages(ctx context.Context, _ *connect.Request[hdlctrlv1.ListMessagesRequest]) (*connect.Response[hdlctrlv1.ListMessagesResponse], error) {
	messages, err := s.muc.ListMessages(ctx)
	if err != nil {
		return nil, convertErr(err)
	}

	out := make([]*hdlctrlv1.Message, 0, len(messages))
	for _, m := range messages {
		out = append(out, messageToProto(m))
	}

	return connect.NewResponse(&hdlctrlv1.ListMessagesResponse{Messages: out}), nil
}

// CreateMessage: system:message.manage が必要.
var _ = registerRPCPermission(
	hdlctrlv1connect.MessageServiceCreateMessageProcedure,
	requireSystemPerm(entity.PermKey_SystemMessageManage),
)

func (s *MessageService) CreateMessage(ctx context.Context, req *connect.Request[hdlctrlv1.CreateMessageRequest]) (*connect.Response[hdlctrlv1.CreateMessageResponse], error) {
	msg, err := s.muc.CreateMessage(ctx, usecase.CreateMessageParams{
		Title:   req.Msg.GetTitle(),
		Body:    req.Msg.GetBody(),
		GroupID: req.Msg.GroupId,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrMessageTitleRequired) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.CreateMessageResponse{Message: messageToProto(msg)}), nil
}

// UpdateMessage: system:message.manage が必要.
var _ = registerRPCPermission(
	hdlctrlv1connect.MessageServiceUpdateMessageProcedure,
	requireSystemPerm(entity.PermKey_SystemMessageManage),
)

func (s *MessageService) UpdateMessage(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateMessageRequest]) (*connect.Response[hdlctrlv1.UpdateMessageResponse], error) {
	msg, err := s.muc.UpdateMessage(ctx, usecase.UpdateMessageParams{
		ID:      req.Msg.GetMessageId(),
		Title:   req.Msg.GetTitle(),
		Body:    req.Msg.GetBody(),
		GroupID: req.Msg.GroupId,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrMessageTitleRequired) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateMessageResponse{Message: messageToProto(msg)}), nil
}

// DeleteMessage: system:message.manage が必要.
var _ = registerRPCPermission(
	hdlctrlv1connect.MessageServiceDeleteMessageProcedure,
	requireSystemPerm(entity.PermKey_SystemMessageManage),
)

func (s *MessageService) DeleteMessage(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteMessageRequest]) (*connect.Response[hdlctrlv1.DeleteMessageResponse], error) {
	if err := s.muc.DeleteMessage(ctx, req.Msg.GetMessageId()); err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteMessageResponse{}), nil
}

func messageToProto(m *entity.Message) *hdlctrlv1.Message {
	p := &hdlctrlv1.Message{
		Id:            m.ID,
		Title:         m.Title,
		Body:          m.Body,
		GroupId:       m.GroupID,
		GroupName:     m.GroupName,
		CreatedBy:     m.CreatedBy,
		LastUpdatedBy: m.LastUpdatedBy,
	}
	if !m.CreatedAt.IsZero() {
		p.CreatedAt = timestamppb.New(m.CreatedAt)
	}

	if !m.UpdatedAt.IsZero() {
		p.UpdatedAt = timestamppb.New(m.UpdatedAt)
	}

	return p
}
