package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/logging"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
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
	srepo  port.SessionRepository
	hhuc   *usecase.HeadlessHostUsecase
	hauc   *usecase.HeadlessAccountUsecase
	suc    *usecase.SessionUsecase
}

func NewControllerService(hhrepo port.HeadlessHostRepository, srepo port.SessionRepository, hhuc *usecase.HeadlessHostUsecase, hauc *usecase.HeadlessAccountUsecase, suc *usecase.SessionUsecase) *ControllerService {
	return &ControllerService{
		hhrepo: hhrepo,
		srepo:  srepo,
		hhuc:   hhuc,
		hauc:   hauc,
		suc:    suc,
	}
}

func (c *ControllerService) NewHandler() (string, http.Handler) {
	interceptors := connect.WithInterceptors(logging.NewErrorLogInterceptor(), auth.NewAuthInterceptor())
	return hdlctrlv1connect.NewControllerServiceHandler(c, interceptors)
}

// ListHeadlessHostImageTags implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListHeadlessHostImageTags(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessHostImageTagsRequest]) (*connect.Response[hdlctrlv1.ListHeadlessHostImageTagsResponse], error) {
	tags, err := c.hhrepo.ListContainerTags(ctx, nil)
	if err != nil {
		return nil, convertErr(err)
	}
	protoTags := make([]*hdlctrlv1.ListHeadlessHostImageTagsResponse_ContainerImage, 0, len(tags))
	for _, tag := range tags {
		protoTags = append(protoTags, &hdlctrlv1.ListHeadlessHostImageTagsResponse_ContainerImage{
			Tag:             tag.Tag,
			ResoniteVersion: tag.ResoniteVersion,
			IsPrerelease:    tag.IsPreRelease,
			AppVersion:      tag.AppVersion,
		})
	}
	res := connect.NewResponse(&hdlctrlv1.ListHeadlessHostImageTagsResponse{
		Tags: protoTags,
	})

	return res, nil
}

// StartHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.StartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.StartHeadlessHostResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.HeadlessAccountId)
	if err != nil {
		return nil, convertErr(err)
	}

	params := port.HeadlessHostStartParams{
		Name:              req.Msg.Name,
		HeadlessAccount:   *account,
		ContainerImageTag: req.Msg.GetImageTag(),
		StartupConfig:     req.Msg.StartupConfig,
	}
	if req.Msg.AutoUpdatePolicy != nil && req.Msg.GetAutoUpdatePolicy() != hdlctrlv1.HeadlessHostAutoUpdatePolicy_HEADLESS_HOST_AUTO_UPDATE_POLICY_UNKNOWN {
		params.AutoUpdatePolicy = entity.HostAutoUpdatePolicy(req.Msg.GetAutoUpdatePolicy())
	}
	if req.Msg.Memo != nil {
		params.Memo = req.Msg.GetMemo()
	}
	hostId, err := c.hhuc.HeadlessHostStart(ctx, params, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.StartHeadlessHostResponse{
		HostId: hostId,
	})
	return res, nil
}

// CreateHeadlessAccount implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) CreateHeadlessAccount(ctx context.Context, req *connect.Request[hdlctrlv1.CreateHeadlessAccountRequest]) (*connect.Response[hdlctrlv1.CreateHeadlessAccountResponse], error) {
	err := c.hauc.CreateHeadlessAccount(ctx, req.Msg.Credential, req.Msg.Password)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.CreateHeadlessAccountResponse{})
	return res, nil
}

// ListHeadlessAccounts implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListHeadlessAccounts(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessAccountsRequest]) (*connect.Response[hdlctrlv1.ListHeadlessAccountsResponse], error) {
	list, err := c.hauc.ListHeadlessAccounts(ctx)
	if err != nil {
		return nil, convertErr(err)
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

// DeleteHeadlessAccount implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) DeleteHeadlessAccount(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteHeadlessAccountRequest]) (*connect.Response[hdlctrlv1.DeleteHeadlessAccountResponse], error) {
	err := c.hauc.DeleteHeadlessAccount(ctx, req.Msg.AccountId)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteHeadlessAccountResponse{}), nil
}

// UpdateHeadlessAccountCredentials implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateHeadlessAccountCredentials(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessAccountCredentialsRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessAccountCredentialsResponse], error) {
	err := c.hauc.UpdateHeadlessAccountCredentials(ctx, req.Msg.AccountId, req.Msg.Credential, req.Msg.Password)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateHeadlessAccountCredentialsResponse{}), nil
}

// GetHeadlessAccountStorageInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetHeadlessAccountStorageInfo(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessAccountStorageInfoRequest]) (*connect.Response[hdlctrlv1.GetHeadlessAccountStorageInfoResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.AccountId)
	if err != nil {
		return nil, convertErr(err)
	}
	userSession, err := skyfrost.UserLogin(ctx, account.Credential, account.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to login to user: %w", err))
	}
	storageInfo, err := userSession.GetStorage(ctx, account.ResoniteID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get storage info for user: %w", err))
	}

	res := connect.NewResponse(&hdlctrlv1.GetHeadlessAccountStorageInfoResponse{
		StorageQuotaBytes: storageInfo.QuotaBytes,
		StorageUsedBytes:  storageInfo.UsedBytes,
	})
	return res, nil
}

// RefetchHeadlessAccountInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) RefetchHeadlessAccountInfo(ctx context.Context, req *connect.Request[hdlctrlv1.RefetchHeadlessAccountInfoRequest]) (*connect.Response[hdlctrlv1.RefetchHeadlessAccountInfoResponse], error) {
	err := c.hauc.RefetchHeadlessAccountInfo(ctx, req.Msg.AccountId)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.RefetchHeadlessAccountInfoResponse{}), nil
}

// AcceptFriendRequests implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) AcceptFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.AcceptFriendRequestsRequest]) (*connect.Response[hdlctrlv1.AcceptFriendRequestsResponse], error) {
	hosts, err := c.hhrepo.ListRunningByAccount(ctx, req.Msg.HeadlessAccountId)
	if err != nil {
		return nil, convertErr(err)
	}
	if len(hosts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("このアカウントで起動中のヘッドレスホストが必要です"))
	}
	conn, err := c.hhrepo.GetRpcClient(ctx, hosts[0].ID)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	ids := []string{
		req.Msg.TargetUserId,
	}
	_, err = conn.AcceptFriendRequests(ctx, &headlessv1.AcceptFriendRequestsRequest{
		UserIds: ids,
	})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.AcceptFriendRequestsResponse{})
	return res, nil
}

// GetFriendRequests implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.GetFriendRequestsRequest]) (*connect.Response[hdlctrlv1.GetFriendRequestsResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.HeadlessAccountId)
	if err != nil {
		return nil, convertErr(err)
	}
	userSession, err := skyfrost.UserLogin(ctx, account.Credential, account.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to login to user: %w", err))
	}
	contacts, err := userSession.GetContacts(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get friend requests: %w", err))
	}
	requestedContacts := make([]*hdlctrlv1.UserInfo, 0)
	for _, contact := range contacts {
		if contact.Status == "Requested" {
			requestedContacts = append(requestedContacts, &hdlctrlv1.UserInfo{
				Id:      contact.Id,
				Name:    contact.Username,
				IconUrl: contact.Profile.IconUrl,
			})
		}
	}

	return connect.NewResponse(&hdlctrlv1.GetFriendRequestsResponse{
		RequestedContacts: requestedContacts,
	}), nil
}

// RestartHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) RestartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.RestartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.RestartHeadlessHostResponse], error) {
	var newTag *string
	if req.Msg.WithUpdate {
		str := "latestRelease"
		newTag = &str
	} else if req.Msg.GetWithImageTag() != "" {
		newTag = req.Msg.WithImageTag
	}
	timeout := 10 * 60
	if req.Msg.TimeoutSeconds != nil {
		timeout = int(req.Msg.GetTimeoutSeconds())
	}
	err := c.hhuc.HeadlessHostRestart(ctx, req.Msg.HostId, newTag, req.Msg.WithWorldRestart, timeout)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.RestartHeadlessHostResponse{
		NewHostId: &req.Msg.HostId,
	})
	return res, nil
}

// UpdateHeadlessHostSettings implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateHeadlessHostSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessHostSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessHostSettingsResponse], error) {
	host, err := c.hhuc.HeadlessHostGet(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertErr(err)
	}

	if req.Msg.Name != nil {
		err := c.hhrepo.Rename(ctx, req.Msg.HostId, req.Msg.GetName())
		if err != nil {
			return nil, convertErr(err)
		}
	}

	hasUpdateReq := false
	updateReq := &headlessv1.UpdateHostSettingsRequest{}
	settings := host.HostSettings
	if req.Msg.TickRate != nil {
		hasUpdateReq = true
		tickRate := req.Msg.GetTickRate()
		updateReq.TickRate = &tickRate
		settings.TickRate = tickRate
	}
	if req.Msg.MaxConcurrentAssetTransfers != nil {
		hasUpdateReq = true
		maxConcurrentAssetTransfers := req.Msg.GetMaxConcurrentAssetTransfers()
		updateReq.MaxConcurrentAssetTransfers = &maxConcurrentAssetTransfers
		settings.MaxConcurrentAssetTransfers = maxConcurrentAssetTransfers
	}
	if req.Msg.UsernameOverride != nil {
		hasUpdateReq = true
		usernameOverride := req.Msg.GetUsernameOverride()
		updateReq.UsernameOverride = &usernameOverride
		settings.UsernameOverride = &usernameOverride
	}
	if req.Msg.UpdateAutoSpawnItems {
		hasUpdateReq = true
		updateReq.UpdateAutoSpawnItems = true
		updateReq.AutoSpawnItems = req.Msg.GetAutoSpawnItems()
		settings.AutoSpawnItems = req.Msg.AutoSpawnItems
	}
	if req.Msg.UniverseId != nil {
		hasUpdateReq = true
		// 実行中のホストのUniverseIDの更新は未対応
		settings.UniverseID = req.Msg.UniverseId
	}

	if hasUpdateReq {
		if host.Status == entity.HeadlessHostStatus_RUNNING {
			conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
			if err != nil {
				return nil, convertRpcClientErr(err)
			}
			_, err = conn.UpdateHostSettings(ctx, updateReq)
			if err != nil {
				return nil, convertRpcClientErr(err)
			}
			updated, err := conn.GetStartupConfigToRestore(ctx, &headlessv1.GetStartupConfigToRestoreRequest{
				IncludeStartWorlds: false,
			})
			if err != nil {
				return nil, convertRpcClientErr(err)
			}
			err = c.hhrepo.UpdateHostSettings(ctx, req.Msg.HostId, converter.HeadlessHostSettingsProtoToEntity(updated.StartupConfig))
			if err != nil {
				return nil, convertErr(err)
			}
		} else {
			err = c.hhrepo.UpdateHostSettings(ctx, req.Msg.HostId, &settings)
			if err != nil {
				return nil, convertErr(err)
			}
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
		return nil, convertErr(err)
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
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.ShutdownHeadlessHostResponse{})
	return res, nil
}

// KillHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) KillHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.KillHeadlessHostRequest]) (*connect.Response[hdlctrlv1.KillHeadlessHostResponse], error) {
	err := c.hhuc.HeadlessHostKill(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.KillHeadlessHostResponse{})
	return res, nil
}

// AllowHostAccess implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) AllowHostAccess(ctx context.Context, req *connect.Request[hdlctrlv1.AllowHostAccessRequest]) (*connect.Response[hdlctrlv1.AllowHostAccessResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	_, err = conn.AllowHostAccess(ctx, req.Msg.Request)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.AllowHostAccessResponse{})
	return res, nil
}

// DenyHostAccess implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) DenyHostAccess(ctx context.Context, req *connect.Request[hdlctrlv1.DenyHostAccessRequest]) (*connect.Response[hdlctrlv1.DenyHostAccessResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	_, err = conn.DenyHostAccess(ctx, req.Msg.Request)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.DenyHostAccessResponse{})
	return res, nil
}

// BanUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) BanUser(ctx context.Context, req *connect.Request[hdlctrlv1.BanUserRequest]) (*connect.Response[hdlctrlv1.BanUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	_, err = conn.BanUser(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.BanUserResponse{})
	return res, nil
}

// KickUser implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) KickUser(ctx context.Context, req *connect.Request[hdlctrlv1.KickUserRequest]) (*connect.Response[hdlctrlv1.KickUserResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	_, err = conn.KickUser(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.KickUserResponse{})
	return res, nil
}

// SearchUserInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SearchUserInfo(ctx context.Context, req *connect.Request[hdlctrlv1.SearchUserInfoRequest]) (*connect.Response[headlessv1.SearchUserInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	headlessRes, err := conn.SearchUserInfo(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(headlessRes)
	return res, nil
}

// FetchWorldInfo implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) FetchWorldInfo(ctx context.Context, req *connect.Request[hdlctrlv1.FetchWorldInfoRequest]) (*connect.Response[headlessv1.FetchWorldInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
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
		return nil, convertErr(err)
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
		return nil, convertErr(err)
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

// DeleteHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) DeleteHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteHeadlessHostRequest]) (*connect.Response[hdlctrlv1.DeleteHeadlessHostResponse], error) {
	err := c.hhuc.HeadlessHostDelete(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteHeadlessHostResponse{}), nil
}

// GetSessionDetails implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) GetSessionDetails(ctx context.Context, req *connect.Request[hdlctrlv1.GetSessionDetailsRequest]) (*connect.Response[hdlctrlv1.GetSessionDetailsResponse], error) {
	s, err := c.suc.GetSession(ctx, req.Msg.SessionId)
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
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}
	headlessRes, err := conn.ListUsersInSession(ctx, &headlessv1.ListUsersInSessionRequest{SessionId: req.Msg.SessionId})
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.ListUsersInSessionResponse{
		Users: headlessRes.Users,
	})
	return res, nil
}

// SaveSessionWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) SaveSessionWorld(ctx context.Context, req *connect.Request[hdlctrlv1.SaveSessionWorldRequest]) (*connect.Response[hdlctrlv1.SaveSessionWorldResponse], error) {
	var saveMode usecase.SaveMode
	switch req.Msg.SaveMode {
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE:
		saveMode = usecase.SaveMode_OVERWRITE
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_SAVE_AS:
		saveMode = usecase.SaveMode_SAVE_AS
	case hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_COPY:
		saveMode = usecase.SaveMode_COPY
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid save mode: %s", req.Msg.SaveMode.String()))
	}

	savedRecordUrl, err := c.suc.SaveSessionWorld(ctx, req.Msg.SessionId, saveMode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&hdlctrlv1.SaveSessionWorldResponse{
		SavedRecordUrl: &savedRecordUrl,
	})
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
		return nil, convertRpcClientErr(err)
	}
	headlessRes, err := conn.UpdateUserRole(ctx, req.Msg.Parameters)
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.UpdateUserRoleResponse{
		Role: headlessRes.Role,
	})
	return res, nil
}

// StartWorld implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StartWorld(ctx context.Context, req *connect.Request[hdlctrlv1.StartWorldRequest]) (*connect.Response[hdlctrlv1.StartWorldResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	openedSession, err := c.suc.StartSession(ctx, req.Msg.HostId, &claims.UserID, req.Msg.Parameters, &req.Msg.Memo)
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
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.HostId)
	if err != nil {
		return nil, convertRpcClientErr(err)
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
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.InviteUserResponse{})
	return res, nil
}

// StopSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) StopSession(ctx context.Context, req *connect.Request[hdlctrlv1.StopSessionRequest]) (*connect.Response[hdlctrlv1.StopSessionResponse], error) {
	err := c.suc.StopSession(ctx, req.Msg.SessionId)
	if err != nil {
		return nil, convertErr(err)
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
		protoSessions = append(protoSessions, converter.SessionEntityToProto(session))
	}
	res := connect.NewResponse(&hdlctrlv1.SearchSessionsResponse{
		Sessions: protoSessions,
	})

	return res, nil
}

// DeleteEndedSession implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) DeleteEndedSession(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteEndedSessionRequest]) (*connect.Response[hdlctrlv1.DeleteEndedSessionResponse], error) {
	err := c.suc.DeleteSession(ctx, req.Msg.SessionId)
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.DeleteEndedSessionResponse{})
	return res, nil
}

// UpdateSessionExtraSettings implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) UpdateSessionExtraSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateSessionExtraSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateSessionExtraSettingsResponse], error) {
	// TODO: いい感じにusecaseに移動する
	s, err := c.suc.GetSession(ctx, req.Msg.SessionId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if req.Msg.AutoUpgrade != nil {
		s.AutoUpgrade = *req.Msg.AutoUpgrade
	}
	if req.Msg.Memo != nil {
		s.Memo = *req.Msg.Memo
	}
	err = c.srepo.Upsert(ctx, s)
	if err != nil {
		return nil, convertErr(err)
	}
	res := connect.NewResponse(&hdlctrlv1.UpdateSessionExtraSettingsResponse{})

	return res, nil
}

// convertErr converts domain errors to appropriate Connect RPC error codes
func convertErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// convertRpcClientErr converts domain errors to appropriate Connect RPC error codes for RPC client operations
func convertRpcClientErr(err error) error {
	if err == nil {
		return nil
	}
	return connect.NewError(connect.CodeInternal, err)
}
