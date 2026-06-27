package rpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/resonitelink"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BanUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) BanUser(ctx context.Context, req *connect.Request[hdlctrlv1.BanUserRequest]) (*connect.Response[hdlctrlv1.BanUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	_, err = conn.BanUser(ctx, req.Msg.GetParameters())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.BanUserResponse{})

	return res, nil
}

// KickUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) KickUser(ctx context.Context, req *connect.Request[hdlctrlv1.KickUserRequest]) (*connect.Response[hdlctrlv1.KickUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	_, err = conn.KickUser(ctx, req.Msg.GetParameters())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.KickUserResponse{})

	return res, nil
}

// GetSessionDetails implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetSessionDetails(ctx context.Context, req *connect.Request[hdlctrlv1.GetSessionDetailsRequest]) (*connect.Response[hdlctrlv1.GetSessionDetailsResponse], error) {
	s, err := c.suc.GetSession(ctx, req.Msg.GetSessionId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.GetSessionDetailsResponse{
		Session: converter.SessionEntityToProto(s),
	})

	return res, nil
}

// ListUsersInSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListUsersInSession(ctx context.Context, req *connect.Request[hdlctrlv1.ListUsersInSessionRequest]) (*connect.Response[hdlctrlv1.ListUsersInSessionResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.ListUsersInSession(ctx, &headlessv1.ListUsersInSessionRequest{SessionId: req.Msg.GetSessionId()})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.ListUsersInSessionResponse{
		Users: headlessRes.GetUsers(),
	})

	return res, nil
}

// SaveSessionWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SaveSessionWorld(ctx context.Context, req *connect.Request[hdlctrlv1.SaveSessionWorldRequest]) (*connect.Response[hdlctrlv1.SaveSessionWorldResponse], error) {
	var saveMode usecase.SaveMode

	switch req.Msg.GetSaveMode() {
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE:
		saveMode = usecase.SaveMode_OVERWRITE
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_SAVE_AS:
		saveMode = usecase.SaveMode_SAVE_AS
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_COPY:
		saveMode = usecase.SaveMode_COPY
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_UNKNOWN:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid save mode: %s", req.Msg.GetSaveMode().String()))
	}

	savedRecordUrl, err := c.suc.SaveSessionWorld(ctx, req.Msg.GetSessionId(), saveMode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.SaveSessionWorldResponse{
		SavedRecordUrl: &savedRecordUrl,
	})

	return res, nil
}

// PrepareSessionWorldDownload implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) PrepareSessionWorldDownload(ctx context.Context, req *connect.Request[hdlctrlv1.PrepareSessionWorldDownloadRequest]) (*connect.Response[hdlctrlv1.PrepareSessionWorldDownloadResponse], error) {
	if req.Msg.GetFormat() == headlessv1.WorldBinaryFormat_WORLD_BINARY_FORMAT_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("format is required"))
	}

	url, filename, err := c.buc.PrepareSessionWorldDownload(ctx, req.Msg.GetSessionId(), req.Msg.GetFormat())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.PrepareSessionWorldDownloadResponse{
		DownloadUrl: url,
		Filename:    filename,
	}), nil
}

// UpdateSessionParameters implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateSessionParameters(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionParametersRequest]) (*connect.Response[hdlctrlv1.UpdateSessionParametersResponse], error) {
	err := c.suc.UpdateSessionParameters(ctx, req.Msg.GetParameters().GetSessionId(), req.Msg.GetParameters())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateSessionParametersResponse{})

	return res, nil
}

// UpdateUserRole implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateUserRole(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateUserRoleRequest]) (*connect.Response[hdlctrlv1.UpdateUserRoleResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.UpdateUserRole(ctx, req.Msg.GetParameters())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateUserRoleResponse{
		Role: headlessRes.GetRole(),
	})

	return res, nil
}

// StartWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartWorld(ctx context.Context, req *connect.Request[hdlctrlv1.StartWorldRequest]) (*connect.Response[hdlctrlv1.StartWorldResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	openedSession, err := c.suc.StartSession(ctx, req.Msg.GetHostId(), &claims.UserID, req.Msg.GetParameters(), &req.Msg.Memo)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.StartWorldResponse{
		OpenedSession: converter.SessionEntityToProto(openedSession),
	})

	return res, nil
}

// InviteUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) InviteUser(ctx context.Context, req *connect.Request[hdlctrlv1.InviteUserRequest]) (*connect.Response[hdlctrlv1.InviteUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	hreq := &headlessv1.InviteUserRequest{
		SessionId: req.Msg.GetSessionId(),
	}
	if req.Msg.GetUserId() != "" {
		hreq.User = &headlessv1.InviteUserRequest_UserId{UserId: req.Msg.GetUserId()}
	} else {
		hreq.User = &headlessv1.InviteUserRequest_UserName{UserName: req.Msg.GetUserName()}
	}

	_, err = conn.InviteUser(ctx, hreq)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.InviteUserResponse{})

	return res, nil
}

// IssueResoniteLinkConnection implements hdlctrlv1connect.ControllerServiceHandler.
// 認証済みユーザに対し ResoniteLink WebSocket 接続用の path?query を返す。
// 返される ws_path は host を含まない相対パスで、クライアントが現在の origin を補完して使う。
func (c *ControllerService) IssueResoniteLinkConnection(ctx context.Context, req *connect.Request[hdlctrlv1.IssueResoniteLinkConnectionRequest]) (*connect.Response[hdlctrlv1.IssueResoniteLinkConnectionResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if req.Msg.GetSessionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("session_id is required"))
	}

	token, expiresAt, err := c.suc.IssueResoniteLinkToken(ctx, req.Msg.GetSessionId(), claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.IssueResoniteLinkConnectionResponse{
		WsPath:    resonitelink.BuildWSPath(token),
		ExpiresAt: timestamppb.New(expiresAt),
	})

	return res, nil
}

// StopSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StopSession(ctx context.Context, req *connect.Request[hdlctrlv1.StopSessionRequest]) (*connect.Response[hdlctrlv1.StopSessionResponse], error) {
	err := c.suc.StopSession(ctx, req.Msg.GetSessionId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.StopSessionResponse{})

	return res, nil
}

// SearchSessions implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SearchSessions(ctx context.Context, req *connect.Request[hdlctrlv1.SearchSessionsRequest]) (*connect.Response[hdlctrlv1.SearchSessionsResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	filter := usecase.SearchSessionsFilter{
		HostID:    req.Msg.GetParameters().HostId,
		PageIndex: pageIndex,
		PageSize:  pageSize,
	}

	if req.Msg.GetParameters() != nil && req.Msg.GetParameters().Status != nil {
		s := entity.SessionStatus(int32(req.Msg.GetParameters().GetStatus().Number()))
		filter.Status = &s
	}

	result, err := c.suc.SearchSessions(ctx, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	protoSessions := make([]*hdlctrlv1.Session, 0, len(result.Sessions))
	for _, session := range result.Sessions {
		protoSessions = append(protoSessions, converter.SessionEntityToProto(session))
	}

	res := connect.NewResponse(&hdlctrlv1.SearchSessionsResponse{
		Sessions: protoSessions,
		Page: &hdlctrlv1.PageResponse{
			TotalCount: result.TotalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
		},
	})

	return res, nil
}

// DeleteEndedSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) DeleteEndedSession(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteEndedSessionRequest]) (*connect.Response[hdlctrlv1.DeleteEndedSessionResponse], error) {
	err := c.suc.DeleteSession(ctx, req.Msg.GetSessionId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.DeleteEndedSessionResponse{})

	return res, nil
}

// UpdateSessionExtraSettings implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateSessionExtraSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionExtraSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateSessionExtraSettingsResponse], error) {
	if err := c.suc.UpdateSessionExtraSettings(ctx, req.Msg.GetSessionId(), req.Msg.AutoUpgrade, req.Msg.Memo); err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateSessionExtraSettingsResponse{}), nil
}
