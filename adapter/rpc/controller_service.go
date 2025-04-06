package rpc

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var _ hdlctrlv1connect.ControllerServiceHandler = (*ControllerService)(nil)

type ControllerService struct {
	hhrepo port.HeadlessHostRepository
	hhuc   *usecase.HeadlessHostUsecase
	hauc   *usecase.HeadlessAccountUsecase
	suc    *usecase.SessionUsecase
}

func NewControllerService(hhrepo port.HeadlessHostRepository, hhuc *usecase.HeadlessHostUsecase, hauc *usecase.HeadlessAccountUsecase, suc *usecase.SessionUsecase) *ControllerService {
	return &ControllerService{
		hhrepo: hhrepo,
		hhuc:   hhuc,
		hauc:   hauc,
		suc:    suc,
	}
}

func (c *ControllerService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(auth.NewAuthInterceptor())
	return hdlctrlv1connect.NewControllerServiceHandler(c, interceptors)
}

// StartHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.StartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.StartHeadlessHostResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.HeadlessAccountId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	hostId, err := c.hhuc.HeadlessHostStart(ctx, port.HeadlessHostStartParams{
		Name:                      req.Msg.Name,
		HeadlessAccountCredential: account.Credential,
		HeadlessAccountPassword:   account.Password,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.StartHeadlessHostResponse{
		HostId: hostId,
	})
	return res, nil
}

// CreateHeadlessAccount implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) CreateHeadlessAccount(ctx context.Context, req *connect.Request[hdlctrlv1.CreateHeadlessAccountRequest]) (*connect.Response[hdlctrlv1.CreateHeadlessAccountResponse], error) {
	err := c.hauc.CreateHeadlessAccount(ctx, req.Msg.ResoniteUserId, req.Msg.Credential, req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.CreateHeadlessAccountResponse{})
	return res, nil
}

// ListHeadlessAccounts implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListHeadlessAccounts(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessAccountsRequest]) (*connect.Response[hdlctrlv1.ListHeadlessAccountsResponse], error) {
	list, err := c.hauc.ListHeadlessAccounts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	protoAccounts := make([]*hdlctrlv1.HeadlessAccount, 0, len(list))
	for _, account := range list {
		a := &hdlctrlv1.HeadlessAccount{
			UserId: account.ResoniteID,
		}
		if account.LastDisplayName != nil {
			a.UserName = *account.LastDisplayName
		} else {
			a.UserName = ""
		}
		if account.LastIconUrl != nil {
			a.IconUrl = *account.LastIconUrl
		} else {
			a.IconUrl = ""
		}
		protoAccounts = append(protoAccounts, a)
	}

	res := connect.NewResponse(&hdlctrlv1.ListHeadlessAccountsResponse{Accounts: protoAccounts})
	return res, nil
}

// AcceptFriendRequests implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) AcceptFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.AcceptFriendRequestsRequest]) (*connect.Response[hdlctrlv1.AcceptFriendRequestsResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_, err = conn.AcceptFriendRequests(ctx, &headlessv1.AcceptFriendRequestsRequest{
		UserIds: req.Msg.UserIds,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.AcceptFriendRequestsResponse{})
	return res, nil
}

// GetFriendRequests implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.GetFriendRequestsRequest]) (*connect.Response[headlessv1.GetFriendRequestsResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.GetFriendRequests(ctx, &headlessv1.GetFriendRequestsRequest{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(headlessRes), nil
}

// RestartHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) RestartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.RestartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.RestartHeadlessHostResponse], error) {
	// TODO: うまい具合に非同期化する
	newId, err := c.hhuc.HeadlessHostRestart(ctx, req.Msg.HostId, req.Msg.WithUpdate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.RestartHeadlessHostResponse{
		NewHostId: &newId,
	})
	return res, nil
}

// PullLatestHostImage implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) PullLatestHostImage(ctx context.Context, req *connect.Request[hdlctrlv1.PullLatestHostImageRequest]) (*connect.Response[hdlctrlv1.PullLatestHostImageResponse], error) {
	logs, err := c.hhuc.PullLatestHostImage(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.PullLatestHostImageResponse{
		Logs: logs,
	})
	return res, nil
}

// UpdateHeadlessHostSettings implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateHeadlessHostSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessHostSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessHostSettingsResponse], error) {
	if req.Msg.Name != nil {
		err := c.hhrepo.Rename(ctx, req.Msg.HostId, req.Msg.GetName())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateHeadlessHostSettingsResponse{})
	return res, nil
}

// GetHeadlessHostLogs implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetHeadlessHostLogs(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessHostLogsRequest]) (*connect.Response[hdlctrlv1.GetHeadlessHostLogsResponse], error) {
	until := req.Msg.GetUntil()
	untilStr := ""
	if until != nil {
		untilStr = fmt.Sprintf("%d", until.AsTime().Unix())
	}
	since := req.Msg.GetSince()
	sinceStr := ""
	if since != nil {
		sinceStr = fmt.Sprintf("%d", since.AsTime().Unix())
	}
	logs, err := c.hhuc.HeadlessHostGetLogs(ctx, req.Msg.HostId, untilStr, sinceStr, req.Msg.GetLimit())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoLogs := make([]*hdlctrlv1.GetHeadlessHostLogsResponse_Log, 0, len(logs))
	for _, log := range logs {
		protoLogs = append(protoLogs, &hdlctrlv1.GetHeadlessHostLogsResponse_Log{
			Timestamp: timestamppb.New(time.Unix(log.Timestamp, 0)),
			IsError:   log.IsError,
			Body:      log.Body,
		})
	}
	res := connect.NewResponse(&hdlctrlv1.GetHeadlessHostLogsResponse{
		Logs: protoLogs,
	})
	return res, nil
}

// ShutdownHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ShutdownHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.ShutdownHeadlessHostRequest]) (*connect.Response[hdlctrlv1.ShutdownHeadlessHostResponse], error) {
	err := c.hhuc.HeadlessHostShutdown(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.ShutdownHeadlessHostResponse{})
	return res, nil
}

// BanUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) BanUser(ctx context.Context, req *connect.Request[hdlctrlv1.BanUserRequest]) (*connect.Response[hdlctrlv1.BanUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_, err = conn.BanUser(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.BanUserResponse{})
	return res, nil
}

// KickUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) KickUser(ctx context.Context, req *connect.Request[hdlctrlv1.KickUserRequest]) (*connect.Response[hdlctrlv1.KickUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_, err = conn.KickUser(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.KickUserResponse{})
	return res, nil
}

// SearchUserInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SearchUserInfo(ctx context.Context, req *connect.Request[hdlctrlv1.SearchUserInfoRequest]) (*connect.Response[headlessv1.SearchUserInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.SearchUserInfo(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(headlessRes)
	return res, nil
}

// FetchWorldInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) FetchWorldInfo(ctx context.Context, req *connect.Request[hdlctrlv1.FetchWorldInfoRequest]) (*connect.Response[headlessv1.FetchWorldInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.FetchWorldInfo(ctx, &headlessv1.FetchWorldInfoRequest{
		Url: req.Msg.Url,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	res := connect.NewResponse(headlessRes)
	return res, nil
}

// GetHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessHostRequest]) (*connect.Response[hdlctrlv1.GetHeadlessHostResponse], error) {
	host, err := c.hhuc.HeadlessHostGet(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := connect.NewResponse(&hdlctrlv1.GetHeadlessHostResponse{
		Host: converter.HeadlessHostEntityToProto(host),
	})
	return res, nil
}

// ListHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessHostRequest]) (*connect.Response[hdlctrlv1.ListHeadlessHostResponse], error) {
	hosts, err := c.hhuc.HeadlessHostList(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	protoHosts := make([]*hdlctrlv1.HeadlessHost, 0, len(hosts))
	for _, host := range hosts {
		protoHosts = append(protoHosts, converter.HeadlessHostEntityToProto(host))
	}
	res := connect.NewResponse(&hdlctrlv1.ListHeadlessHostResponse{
		Hosts: protoHosts,
	})
	return res, nil
}

// GetSessionDetails implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetSessionDetails(ctx context.Context, req *connect.Request[hdlctrlv1.GetSessionDetailsRequest]) (*connect.Response[hdlctrlv1.GetSessionDetailsResponse], error) {
	s, err := c.suc.GetSession(ctx, req.Msg.SessionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := connect.NewResponse(&hdlctrlv1.GetSessionDetailsResponse{
		Session: s.ToProto(),
	})

	return res, nil
}

// ListUsersInSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListUsersInSession(ctx context.Context, req *connect.Request[hdlctrlv1.ListUsersInSessionRequest]) (*connect.Response[hdlctrlv1.ListUsersInSessionResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.ListUsersInSession(ctx, &headlessv1.ListUsersInSessionRequest{SessionId: req.Msg.SessionId})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.ListUsersInSessionResponse{
		Users: headlessRes.Users,
	})
	return res, nil
}

// SaveSessionWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SaveSessionWorld(ctx context.Context, req *connect.Request[hdlctrlv1.SaveSessionWorldRequest]) (*connect.Response[hdlctrlv1.SaveSessionWorldResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	_, err = conn.SaveSessionWorld(ctx, &headlessv1.SaveSessionWorldRequest{SessionId: req.Msg.SessionId})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.SaveSessionWorldResponse{})
	return res, nil
}

// UpdateSessionParameters implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateSessionParameters(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionParametersRequest]) (*connect.Response[hdlctrlv1.UpdateSessionParametersResponse], error) {
	err := c.suc.UpdateSessionParameters(ctx, req.Msg.Parameters.SessionId, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateSessionParametersResponse{})
	return res, nil
}

// UpdateUserRole implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateUserRole(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateUserRoleRequest]) (*connect.Response[hdlctrlv1.UpdateUserRoleResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	headlessRes, err := conn.UpdateUserRole(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateUserRoleResponse{
		Role: headlessRes.Role,
	})
	return res, nil
}

// StartWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartWorld(ctx context.Context, req *connect.Request[hdlctrlv1.StartWorldRequest]) (*connect.Response[hdlctrlv1.StartWorldResponse], error) {
	openedSession, err := c.suc.StartSession(ctx, req.Msg.HostId, req.Msg.Parameters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.StartWorldResponse{
		OpenedSession: openedSession.ToProto(),
	})
	return res, nil
}

// InviteUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) InviteUser(ctx context.Context, req *connect.Request[hdlctrlv1.InviteUserRequest]) (*connect.Response[hdlctrlv1.InviteUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	hreq := &headlessv1.InviteUserRequest{
		SessionId: req.Msg.SessionId,
	}
	if req.Msg.GetUserId() != "" {
		hreq.User = &headlessv1.InviteUserRequest_UserId{UserId: req.Msg.GetUserId()}
	} else {
		hreq.User = &headlessv1.InviteUserRequest_UserName{UserName: req.Msg.GetUserName()}
	}
	_, err = conn.InviteUser(ctx, hreq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.InviteUserResponse{})
	return res, nil
}

// StopSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StopSession(ctx context.Context, req *connect.Request[hdlctrlv1.StopSessionRequest]) (*connect.Response[hdlctrlv1.StopSessionResponse], error) {
	err := c.suc.StopSession(ctx, req.Msg.SessionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.StopSessionResponse{})
	return res, nil
}

// SearchSessions implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SearchSessions(ctx context.Context, req *connect.Request[hdlctrlv1.SearchSessionsRequest]) (*connect.Response[hdlctrlv1.SearchSessionsResponse], error) {
	filter := usecase.SearchSessionsFilter{
		HostID: req.Msg.Parameters.HostId,
	}
	if req.Msg.Parameters.Status != nil {
		s := entity.SessionStatus(int32(req.Msg.Parameters.Status.Number()))
		filter.Status = &s
	}

	sessions, err := c.suc.SearchSessions(ctx, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	protoSessions := make([]*hdlctrlv1.Session, 0, len(sessions))
	for _, session := range sessions {
		protoSessions = append(protoSessions, session.ToProto())
	}
	res := connect.NewResponse(&hdlctrlv1.SearchSessionsResponse{
		Sessions: protoSessions,
	})

	return res, nil
}
