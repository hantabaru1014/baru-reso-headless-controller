package rpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// AcceptFriendRequests implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: account.group_id に対して account:use.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceAcceptFriendRequestsProcedure,
	checkAccountPermission(entity.PermKey_AccountUse, accountIDFromAcceptFriends),
)

func (c *ControllerService) AcceptFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.AcceptFriendRequestsRequest]) (*connect.Response[hdlctrlv1.AcceptFriendRequestsResponse], error) {
	hosts, err := c.hhrepo.ListRunningByAccount(ctx, req.Msg.GetHeadlessAccountId())
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
		req.Msg.GetTargetUserId(),
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
// 権限: account.group_id に対して account:use.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetFriendRequestsProcedure,
	checkAccountPermission(entity.PermKey_AccountUse, accountIDFromGetFriendRequests),
)

func (c *ControllerService) GetFriendRequests(ctx context.Context, req *connect.Request[hdlctrlv1.GetFriendRequestsRequest]) (*connect.Response[hdlctrlv1.GetFriendRequestsResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	contacts, err := c.skyfrostClient.GetContacts(ctx, account.Credential, account.Password)
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

// SearchUserInfo implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:use (host RPC への委譲読み取り).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceSearchUserInfoProcedure,
	checkHostPermission(entity.PermKey_HostUse, hostIDFromSearchUserInfo),
)

func (c *ControllerService) SearchUserInfo(ctx context.Context, req *connect.Request[hdlctrlv1.SearchUserInfoRequest]) (*connect.Response[headlessv1.SearchUserInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.SearchUserInfo(ctx, req.Msg.GetParameters())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(headlessRes)

	return res, nil
}

// FetchWorldInfo implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:use (host RPC への委譲読み取り).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceFetchWorldInfoProcedure,
	checkHostPermission(entity.PermKey_HostUse, hostIDFromFetchWorldInfo),
)

func (c *ControllerService) FetchWorldInfo(ctx context.Context, req *connect.Request[hdlctrlv1.FetchWorldInfoRequest]) (*connect.Response[headlessv1.FetchWorldInfoResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	headlessRes, err := conn.FetchWorldInfo(ctx, &headlessv1.FetchWorldInfoRequest{
		Url: req.Msg.GetUrl(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	res := connect.NewResponse(headlessRes)

	return res, nil
}

// GetResoniteUser implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: 認証のみ (Resonite 公開プロフィール参照).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetResoniteUserProcedure,
	requireAuthOnly,
)

func (c *ControllerService) GetResoniteUser(ctx context.Context, req *connect.Request[hdlctrlv1.GetResoniteUserRequest]) (*connect.Response[hdlctrlv1.GetResoniteUserResponse], error) {
	userInfo, err := c.skyfrostClient.FetchUserInfo(ctx, req.Msg.GetResoniteId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	res := connect.NewResponse(&hdlctrlv1.GetResoniteUserResponse{
		Id:      userInfo.ID,
		Name:    userInfo.UserName,
		IconUrl: userInfo.IconUrl,
	})

	return res, nil
}

// GetOwnWorlds implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:use (host のアカウントで Cloud にアクセス).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetOwnWorldsProcedure,
	checkHostPermission(entity.PermKey_HostUse, hostIDFromGetOwnWorlds),
)

func (c *ControllerService) GetOwnWorlds(ctx context.Context, req *connect.Request[hdlctrlv1.GetOwnWorldsRequest]) (*connect.Response[hdlctrlv1.GetOwnWorldsResponse], error) {
	host, err := c.hhuc.HeadlessHostGet(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	account, err := c.hauc.GetHeadlessAccount(ctx, host.AccountId)
	if err != nil {
		return nil, convertErr(err)
	}

	result, err := c.skyfrostClient.GetOwnWorlds(ctx, account.Credential, account.Password, int(req.Msg.GetPageIndex()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get own worlds: %w", err))
	}

	records := make([]*hdlctrlv1.SearchWorldsResponse_WorldRecord, 0, len(result.Records))
	for _, r := range result.Records {
		records = append(records, &hdlctrlv1.SearchWorldsResponse_WorldRecord{
			Id:           r.ID,
			OwnerId:      r.OwnerID,
			OwnerName:    r.OwnerName,
			Name:         r.Name,
			Description:  r.Description,
			ThumbnailUrl: r.ThumbnailUri,
			IsFeatured:   r.IsFeatured,
		})
	}

	res := connect.NewResponse(&hdlctrlv1.GetOwnWorldsResponse{
		Records: records,
		HasMore: result.HasMore,
	})

	return res, nil
}

// SearchWorlds implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: 認証のみ (公開ワールド検索).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceSearchWorldsProcedure,
	requireAuthOnly,
)

func (c *ControllerService) SearchWorlds(ctx context.Context, req *connect.Request[hdlctrlv1.SearchWorldsRequest]) (*connect.Response[hdlctrlv1.SearchWorldsResponse], error) {
	result, err := c.skyfrostClient.SearchWorlds(ctx, req.Msg.GetQuery(), req.Msg.GetFeaturedOnly(), int(req.Msg.GetPageIndex()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	records := make([]*hdlctrlv1.SearchWorldsResponse_WorldRecord, 0, len(result.Records))
	for _, r := range result.Records {
		records = append(records, &hdlctrlv1.SearchWorldsResponse_WorldRecord{
			Id:           r.ID,
			OwnerId:      r.OwnerID,
			OwnerName:    r.OwnerName,
			Name:         r.Name,
			Description:  r.Description,
			ThumbnailUrl: r.ThumbnailUri,
			IsFeatured:   r.IsFeatured,
		})
	}

	res := connect.NewResponse(&hdlctrlv1.SearchWorldsResponse{
		Records: records,
		HasMore: result.HasMore,
	})

	return res, nil
}
