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
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BanUser implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して session:write (host RPC への委譲書き込み).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceBanUserProcedure,
	checkHostPermission(entity.PermKey_SessionWrite, hostIDFromBan),
)

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
// 権限: host.group_id に対して session:write (host RPC への委譲書き込み).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceKickUserProcedure,
	checkHostPermission(entity.PermKey_SessionWrite, hostIDFromKick),
)

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
// 権限: session.group_id に対して session:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetSessionDetailsProcedure,
	checkSessionPermission(entity.PermKey_SessionRead, sessionIDFromGetDetails),
)

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
// 権限: host.group_id に対して session:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListUsersInSessionProcedure,
	checkHostPermission(entity.PermKey_SessionRead, hostIDFromListUsersInSession),
)

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
// 権限: session.group_id に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceSaveSessionWorldProcedure,
	checkSessionPermission(entity.PermKey_SessionWrite, sessionIDFromSave),
)

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
// 権限: session.group_id に対して session:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServicePrepareSessionWorldDownloadProcedure,
	checkSessionPermission(entity.PermKey_SessionRead, sessionIDFromPrepareDownload),
)

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
// 権限: session.group_id (parameters.session_id から DB lookup) に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateSessionParametersProcedure,
	checkUpdateSessionParameters,
)

func (c *ControllerService) UpdateSessionParameters(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionParametersRequest]) (*connect.Response[hdlctrlv1.UpdateSessionParametersResponse], error) {
	err := c.suc.UpdateSessionParameters(ctx, req.Msg.GetParameters().GetSessionId(), req.Msg.GetParameters())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateSessionParametersResponse{})

	return res, nil
}

// UpdateUserRole implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して session:write (host RPC への委譲書き込み).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateUserRoleProcedure,
	checkHostPermission(entity.PermKey_SessionWrite, hostIDFromUpdateUserRole),
)

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
// container への StartWorld RPC は時間がかかるため非同期 job 化する.
// 権限: host.group_id に対して host:use + account:use + session:write (同一グループ制約).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceStartWorldProcedure,
	checkStartWorld,
)

func (c *ControllerService) StartWorld(ctx context.Context, req *connect.Request[hdlctrlv1.StartWorldRequest]) (*connect.Response[hdlctrlv1.StartWorldResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// host の group_id を読むためだけに DB lookup. Find は RUNNING host に対して
	// container RPC を起こすため、軽量な GetGroupID を使う.
	hostGroupID, err := c.hhrepo.GetGroupID(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	// group_id を resolve: 未指定なら host のグループに合わせる (同一グループ制約).
	if req.Msg.GroupId == nil || req.Msg.GetGroupId() == "" {
		req.Msg.GroupId = &hostGroupID
	}

	jobID, err := c.ajuc.EnqueueStartSession(ctx, req.Msg, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.StartWorldResponse{JobId: jobID}), nil
}

// InviteUser implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して session:write (host RPC への委譲書き込み).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceInviteUserProcedure,
	checkHostPermission(entity.PermKey_SessionWrite, hostIDFromInviteUser),
)

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
// 権限: session.group_id に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceIssueResoniteLinkConnectionProcedure,
	checkSessionPermission(entity.PermKey_SessionWrite, sessionIDFromIssueLink),
)

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
// container への StopSession RPC は時間がかかるため非同期 job 化する.
// 権限: session.group_id に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceStopSessionProcedure,
	checkSessionPermission(entity.PermKey_SessionWrite, sessionIDFromStop),
)

func (c *ControllerService) StopSession(ctx context.Context, req *connect.Request[hdlctrlv1.StopSessionRequest]) (*connect.Response[hdlctrlv1.StopSessionResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// srepo.Get は DB のみで完結する. SessionUsecase.GetSession は cache hydration や
	// container RPC を起こすので、ここでは使わない.
	if _, err := c.srepo.Get(ctx, req.Msg.GetSessionId()); err != nil {
		return nil, convertErr(err)
	}

	jobID, err := c.ajuc.EnqueueStopSession(ctx, req.Msg, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.StopSessionResponse{JobId: jobID}), nil
}

// SearchSessions implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: handler 側で resolveListGroupFilter により認可する (interceptor は通過のみ).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceSearchSessionsProcedure,
	requireAuthOnly,
)

func (c *ControllerService) SearchSessions(ctx context.Context, req *connect.Request[hdlctrlv1.SearchSessionsRequest]) (*connect.Response[hdlctrlv1.SearchSessionsResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	var requestedGroupID *string
	if p := req.Msg.GetParameters(); p != nil {
		requestedGroupID = p.GroupId
	}

	groupIDs, err := c.resolveListGroupFilter(ctx, requestedGroupID, entity.PermKey_SessionRead)
	if err != nil {
		return nil, err
	}

	filter := usecase.SearchSessionsFilter{
		PageIndex: pageIndex,
		PageSize:  pageSize,
		GroupIDs:  groupIDs,
	}

	if p := req.Msg.GetParameters(); p != nil {
		filter.HostID = p.HostId

		if p.Status != nil {
			s := entity.SessionStatus(int32(p.GetStatus().Number()))
			filter.Status = &s
		}
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
// 権限: session.group_id に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceDeleteEndedSessionProcedure,
	checkSessionPermission(entity.PermKey_SessionWrite, sessionIDFromDelete),
)

func (c *ControllerService) DeleteEndedSession(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteEndedSessionRequest]) (*connect.Response[hdlctrlv1.DeleteEndedSessionResponse], error) {
	err := c.suc.DeleteSession(ctx, req.Msg.GetSessionId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.DeleteEndedSessionResponse{})

	return res, nil
}

// UpdateSessionExtraSettings implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: session.group_id に対して session:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateSessionExtraSettingsProcedure,
	checkSessionPermission(entity.PermKey_SessionWrite, sessionIDFromUpdateExtra),
)

func (c *ControllerService) UpdateSessionExtraSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionExtraSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateSessionExtraSettingsResponse], error) {
	if err := c.suc.UpdateSessionExtraSettings(ctx, req.Msg.GetSessionId(), req.Msg.AutoUpgrade, req.Msg.Memo); err != nil { //nolint:protogetter // optional 3 値を保つため pointer field を直接渡す
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateSessionExtraSettingsResponse{}), nil
}
