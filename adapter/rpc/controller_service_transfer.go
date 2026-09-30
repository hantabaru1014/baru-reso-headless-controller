package rpc

import (
	"context"

	"connectrpc.com/connect"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
)

// TransferResources implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: 必要な permission は移管対象に含まれるリソース種別で決まるため、usecase 側
// (ResourceTransferUsecase.Transfer) で認可する (interceptor は通過のみ).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceTransferResourcesProcedure,
	requireAuthOnly,
)

func (c *ControllerService) TransferResources(ctx context.Context, req *connect.Request[hdlctrlv1.TransferResourcesRequest]) (*connect.Response[hdlctrlv1.TransferResourcesResponse], error) {
	target := usecase.ResourceTransferTarget{}

	switch r := req.Msg.GetResource().(type) {
	case *hdlctrlv1.TransferResourcesRequest_HostId:
		target.HostID = r.HostId
	case *hdlctrlv1.TransferResourcesRequest_SessionId:
		target.SessionID = r.SessionId
	case *hdlctrlv1.TransferResourcesRequest_Account:
		target.AccountGroupID = r.Account.GetGroupId()
		target.AccountID = r.Account.GetAccountId()
	}

	result, err := c.rtuc.Transfer(ctx, target, req.Msg.GetDestinationGroupId(), req.Msg.GetDryRun())
	if err != nil {
		return nil, convertErr(err)
	}

	res := &hdlctrlv1.TransferResourcesResponse{
		SourceGroupId:      result.SourceGroupID,
		DestinationGroupId: result.DestinationGroupID,
		Hosts:              make([]*hdlctrlv1.TransferResourcesResponse_Host, 0, len(result.Hosts)),
		Sessions:           make([]*hdlctrlv1.TransferResourcesResponse_Session, 0, len(result.Sessions)),
	}

	if result.Account != nil {
		res.Account = &hdlctrlv1.TransferResourcesResponse_Account{
			UserId: result.Account.ResoniteID,
			Merged: result.AccountMerged,
		}
		if result.Account.LastDisplayName != nil {
			res.Account.UserName = *result.Account.LastDisplayName
		}
	}

	for _, h := range result.Hosts {
		res.Hosts = append(res.Hosts, &hdlctrlv1.TransferResourcesResponse_Host{Id: h.ID, Name: h.Name})
	}

	for _, s := range result.Sessions {
		res.Sessions = append(res.Sessions, &hdlctrlv1.TransferResourcesResponse_Session{Id: s.ID, Name: s.Name})
	}

	if !req.Msg.GetDryRun() {
		// グループが変わると一覧の絞り込み結果と閲覧可否が変わるため、再フェッチを促す.
		for _, h := range result.Hosts {
			c.publishHostUpdated(h.ID)
		}

		c.publishHostListChanged()
	}

	return connect.NewResponse(res), nil
}
