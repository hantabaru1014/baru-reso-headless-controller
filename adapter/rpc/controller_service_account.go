package rpc

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
)

// CreateHeadlessAccount implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: 解決後の group_id に対して account:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceCreateHeadlessAccountProcedure,
	checkCreateHeadlessAccount,
)

func (c *ControllerService) CreateHeadlessAccount(ctx context.Context, req *connect.Request[hdlctrlv1.CreateHeadlessAccountRequest]) (*connect.Response[hdlctrlv1.CreateHeadlessAccountResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	groupID, err := c.permUC.ResolveGroupIDForUser(ctx, claims.UserID, req.Msg.GetGroupId())
	if err != nil {
		return nil, convertErr(err)
	}

	userID := claims.UserID
	if err := c.hauc.CreateHeadlessAccount(ctx, req.Msg.GetCredential(), req.Msg.GetPassword(), groupID, &userID); err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.CreateHeadlessAccountResponse{})

	return res, nil
}

// ListHeadlessAccounts implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: handler 側で resolveListGroupFilter により認可する (interceptor は通過のみ).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListHeadlessAccountsProcedure,
	requireAuthOnly,
)

func (c *ControllerService) ListHeadlessAccounts(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessAccountsRequest]) (*connect.Response[hdlctrlv1.ListHeadlessAccountsResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	groupIDs, err := c.resolveListGroupFilter(ctx, req.Msg.GroupId, entity.PermKey_AccountRead)
	if err != nil {
		return nil, err
	}

	pageResult, err := c.hauc.ListHeadlessAccountsPaged(ctx, usecase.ListHeadlessAccountsPagedOptions{
		PageIndex: pageIndex,
		PageSize:  pageSize,
		GroupIDs:  groupIDs,
	})
	if err != nil {
		return nil, convertErr(err)
	}

	protoAccounts := make([]*hdlctrlv1.HeadlessAccount, 0, len(pageResult.Accounts))

	for _, account := range pageResult.Accounts {
		a := &hdlctrlv1.HeadlessAccount{
			UserId:    account.ResoniteID,
			GroupId:   account.GroupID,
			CreatedBy: account.CreatedBy,
		}
		if account.LastDisplayName != nil {
			a.UserName = *account.LastDisplayName
		}

		if account.LastIconUrl != nil {
			a.IconUrl = *account.LastIconUrl
		}

		protoAccounts = append(protoAccounts, a)
	}

	res := connect.NewResponse(&hdlctrlv1.ListHeadlessAccountsResponse{
		Accounts: protoAccounts,
		Page: &hdlctrlv1.PageResponse{
			TotalCount: pageResult.TotalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
		},
	})

	return res, nil
}

// DeleteHeadlessAccount implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: account.group_id に対して account:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceDeleteHeadlessAccountProcedure,
	checkAccountPermission(entity.PermKey_AccountWrite, accountIDFromDelete),
)

func (c *ControllerService) DeleteHeadlessAccount(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteHeadlessAccountRequest]) (*connect.Response[hdlctrlv1.DeleteHeadlessAccountResponse], error) {
	err := c.hauc.DeleteHeadlessAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.DeleteHeadlessAccountResponse{}), nil
}

// UpdateHeadlessAccountCredentials implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: account.group_id に対して account:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateHeadlessAccountCredentialsProcedure,
	checkAccountPermission(entity.PermKey_AccountWrite, accountIDFromUpdateCreds),
)

func (c *ControllerService) UpdateHeadlessAccountCredentials(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessAccountCredentialsRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessAccountCredentialsResponse], error) {
	err := c.hauc.UpdateHeadlessAccountCredentials(ctx, req.Msg.GetAccountId(), req.Msg.GetCredential(), req.Msg.GetPassword())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateHeadlessAccountCredentialsResponse{}), nil
}

// GetHeadlessAccountStorageInfo implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: account.group_id に対して account:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetHeadlessAccountStorageInfoProcedure,
	checkAccountPermission(entity.PermKey_AccountRead, accountIDFromStorageInfo),
)

func (c *ControllerService) GetHeadlessAccountStorageInfo(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessAccountStorageInfoRequest]) (*connect.Response[hdlctrlv1.GetHeadlessAccountStorageInfoResponse], error) {
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	storageInfo, err := c.skyfrostClient.GetStorageInfo(ctx, account.Credential, account.Password, account.ResoniteID)
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
// 権限: account.group_id に対して account:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceRefetchHeadlessAccountInfoProcedure,
	checkAccountPermission(entity.PermKey_AccountWrite, accountIDFromRefetch),
)

func (c *ControllerService) RefetchHeadlessAccountInfo(ctx context.Context, req *connect.Request[hdlctrlv1.RefetchHeadlessAccountInfoRequest]) (*connect.Response[hdlctrlv1.RefetchHeadlessAccountInfoResponse], error) {
	err := c.hauc.RefetchHeadlessAccountInfo(ctx, req.Msg.GetAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.RefetchHeadlessAccountInfoResponse{}), nil
}

// UpdateHeadlessAccountIcon implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: account.group_id に対して account:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateHeadlessAccountIconProcedure,
	checkAccountPermission(entity.PermKey_AccountWrite, accountIDFromUpdateIcon),
)

func (c *ControllerService) UpdateHeadlessAccountIcon(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessAccountIconRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessAccountIconResponse], error) {
	newIconUrl, err := c.hauc.UpdateHeadlessAccountIcon(ctx, req.Msg.GetAccountId(), req.Msg.GetIconData())
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.UpdateHeadlessAccountIconResponse{
		NewIconUrl: newIconUrl,
	}), nil
}
