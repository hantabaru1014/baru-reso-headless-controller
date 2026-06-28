package rpc

import (
	"context"
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

// ListHeadlessHostImageTags implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: 認証のみ (image tag 情報は機密でない).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListHeadlessHostImageTagsProcedure,
	requireAuthOnly,
)

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
// ホスト起動は docker pull / コンテナ起動 / RPC ハンドシェイクを伴うため
// 非同期 job として enqueue し、完了は notification.JobCompletedEvent で push する.
// 権限: account.group_id に対して host:write + account:use (同一グループ制約).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceStartHeadlessHostProcedure,
	checkStartHeadlessHost,
)

func (c *ControllerService) StartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.StartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.StartHeadlessHostResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// account の存在確認は受付時に行う (存在しない account への job 化を防ぐ).
	account, err := c.hauc.GetHeadlessAccount(ctx, req.Msg.GetHeadlessAccountId())
	if err != nil {
		return nil, convertErr(err)
	}

	// group_id 解決: 未指定なら account のグループ (同一グループ制約).
	// 指定された場合は account.group_id と一致することを permission interceptor が
	// 検証済み.
	if req.Msg.GroupId == nil || req.Msg.GetGroupId() == "" {
		gid := account.GroupID
		req.Msg.GroupId = &gid
	}

	jobID, err := c.ajuc.EnqueueStartHost(ctx, req.Msg, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.StartHeadlessHostResponse{JobId: jobID}), nil
}

// RestartHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// 再起動も停止 + 起動の compound 操作で時間がかかるため非同期 job 化する.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceRestartHeadlessHostProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromRestart),
)

func (c *ControllerService) RestartHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.RestartHeadlessHostRequest]) (*connect.Response[hdlctrlv1.RestartHeadlessHostResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	// 受付時の存在確認: 即座に NotFound を返したいので enqueue 前に check する.
	// hhrepo.Find は DB のみで完結し、RUNNING ホストへの container RPC roundtrip を起こさない.
	if _, err := c.hhrepo.Find(ctx, req.Msg.GetHostId(), port.HeadlessHostFetchOptions{}); err != nil {
		return nil, convertErr(err)
	}

	jobID, err := c.ajuc.EnqueueRestartHost(ctx, req.Msg, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.RestartHeadlessHostResponse{JobId: jobID}), nil
}

// UpdateHeadlessHostSettings implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceUpdateHeadlessHostSettingsProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromUpdateSettings),
)

func (c *ControllerService) UpdateHeadlessHostSettings(ctx context.Context, req *connect.Request[hdlctrlv1.UpdateHeadlessHostSettingsRequest]) (*connect.Response[hdlctrlv1.UpdateHeadlessHostSettingsResponse], error) {
	host, err := c.hhuc.HeadlessHostGet(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	if req.Msg.Name != nil {
		err := c.hhrepo.Rename(ctx, req.Msg.GetHostId(), req.Msg.GetName())
		if err != nil {
			return nil, convertErr(err)
		}
	}

	if req.Msg.AutoUpdatePolicy != nil &&
		req.Msg.GetAutoUpdatePolicy() != hdlctrlv1.HeadlessHostAutoUpdatePolicy_HEADLESS_HOST_AUTO_UPDATE_POLICY_UNKNOWN {
		err := c.hhrepo.UpdateAutoUpdatePolicy(
			ctx,
			req.Msg.GetHostId(),
			entity.HostAutoUpdatePolicy(req.Msg.GetAutoUpdatePolicy()),
		)
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

	if req.Msg.GetUpdateAutoSpawnItems() {
		hasUpdateReq = true
		updateReq.UpdateAutoSpawnItems = true
		updateReq.AutoSpawnItems = req.Msg.GetAutoSpawnItems()
		settings.AutoSpawnItems = req.Msg.GetAutoSpawnItems()
	}

	if req.Msg.UniverseId != nil {
		hasUpdateReq = true
		// 実行中のホストのUniverseIDの更新は未対応
		settings.UniverseID = req.Msg.UniverseId
	}

	if hasUpdateReq {
		if host.Status == entity.HeadlessHostStatus_RUNNING {
			conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
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

			err = c.hhrepo.UpdateHostSettings(ctx, req.Msg.GetHostId(), converter.HeadlessHostSettingsProtoToEntity(updated.GetStartupConfig()))
			if err != nil {
				return nil, convertErr(err)
			}
		} else {
			err = c.hhrepo.UpdateHostSettings(ctx, req.Msg.GetHostId(), &settings)
			if err != nil {
				return nil, convertErr(err)
			}
		}
	}

	c.publishHostUpdated(req.Msg.GetHostId())

	res := connect.NewResponse(&hdlctrlv1.UpdateHeadlessHostSettingsResponse{})

	return res, nil
}

// GetHeadlessHostLogs implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetHeadlessHostLogsProcedure,
	checkHostPermission(entity.PermKey_HostRead, hostIDFromGetLogs),
)

func (c *ControllerService) GetHeadlessHostLogs(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessHostLogsRequest]) (*connect.Response[hdlctrlv1.GetHeadlessHostLogsResponse], error) {
	// カーソル解析 (ID-based)
	var beforeID, afterID int64

	switch cursor := req.Msg.GetCursor().(type) {
	case *hdlctrlv1.GetHeadlessHostLogsRequest_BeforeId:
		beforeID = cursor.BeforeId
	case *hdlctrlv1.GetHeadlessHostLogsRequest_AfterId:
		afterID = cursor.AfterId
	}

	result, err := c.hhuc.HeadlessHostGetLogs(ctx, usecase.HeadlessHostGetLogsParams{
		HostID:     req.Msg.GetHostId(),
		InstanceID: req.Msg.GetInstanceId(),
		Limit:      req.Msg.GetLimit(),
		BeforeID:   beforeID,
		AfterID:    afterID,
	})
	if err != nil {
		return nil, convertErr(err)
	}

	protoLogs := make([]*hdlctrlv1.GetHeadlessHostLogsResponse_Log, 0, len(result.Logs))
	for _, log := range result.Logs {
		protoLogs = append(protoLogs, &hdlctrlv1.GetHeadlessHostLogsResponse_Log{
			Timestamp: timestamppb.New(time.Unix(log.Timestamp, 0)),
			IsError:   log.IsError,
			Body:      log.Body,
			Id:        log.ID,
		})
	}

	res := connect.NewResponse(&hdlctrlv1.GetHeadlessHostLogsResponse{
		Logs:          protoLogs,
		HasMoreBefore: result.HasMoreBefore,
		HasMoreAfter:  result.HasMoreAfter,
	})

	return res, nil
}

// ListHeadlessHostInstances implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListHeadlessHostInstancesProcedure,
	checkHostPermission(entity.PermKey_HostRead, hostIDFromListInstances),
)

func (c *ControllerService) ListHeadlessHostInstances(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessHostInstancesRequest]) (*connect.Response[hdlctrlv1.ListHeadlessHostInstancesResponse], error) {
	instances, err := c.hhuc.HeadlessHostGetInstances(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	protoInstances := make([]*hdlctrlv1.ListHeadlessHostInstancesResponse_Instance, 0, len(instances))

	for _, inst := range instances {
		protoInst := &hdlctrlv1.ListHeadlessHostInstancesResponse_Instance{
			InstanceId: inst.InstanceID,
			LogCount:   inst.LogCount,
			IsCurrent:  inst.IsCurrent,
		}
		if inst.FirstLogAt != nil {
			protoInst.FirstLogAt = timestamppb.New(time.Unix(*inst.FirstLogAt, 0))
		}

		if inst.LastLogAt != nil {
			protoInst.LastLogAt = timestamppb.New(time.Unix(*inst.LastLogAt, 0))
		}

		protoInstances = append(protoInstances, protoInst)
	}

	res := connect.NewResponse(&hdlctrlv1.ListHeadlessHostInstancesResponse{
		Instances: protoInstances,
	})

	return res, nil
}

// ShutdownHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// graceful shutdown は container 終了待ちが入るため非同期 job 化する.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceShutdownHeadlessHostProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromShutdown),
)

func (c *ControllerService) ShutdownHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.ShutdownHeadlessHostRequest]) (*connect.Response[hdlctrlv1.ShutdownHeadlessHostResponse], error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	if _, err := c.hhrepo.Find(ctx, req.Msg.GetHostId(), port.HeadlessHostFetchOptions{}); err != nil {
		return nil, convertErr(err)
	}

	jobID, err := c.ajuc.EnqueueShutdownHost(ctx, req.Msg, &claims.UserID)
	if err != nil {
		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.ShutdownHeadlessHostResponse{JobId: jobID}), nil
}

// KillHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceKillHeadlessHostProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromKill),
)

func (c *ControllerService) KillHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.KillHeadlessHostRequest]) (*connect.Response[hdlctrlv1.KillHeadlessHostResponse], error) {
	err := c.hhuc.HeadlessHostKill(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.KillHeadlessHostResponse{})

	return res, nil
}

// AllowHostAccess implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceAllowHostAccessProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromAllowAccess),
)

func (c *ControllerService) AllowHostAccess(ctx context.Context, req *connect.Request[hdlctrlv1.AllowHostAccessRequest]) (*connect.Response[hdlctrlv1.AllowHostAccessResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	_, err = conn.AllowHostAccess(ctx, req.Msg.GetRequest())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.AllowHostAccessResponse{})

	return res, nil
}

// DenyHostAccess implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceDenyHostAccessProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromDenyAccess),
)

func (c *ControllerService) DenyHostAccess(ctx context.Context, req *connect.Request[hdlctrlv1.DenyHostAccessRequest]) (*connect.Response[hdlctrlv1.DenyHostAccessResponse], error) {
	conn, err := c.hhrepo.GetRpcClient(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	_, err = conn.DenyHostAccess(ctx, req.Msg.GetRequest())
	if err != nil {
		return nil, convertRpcClientErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.DenyHostAccessResponse{})

	return res, nil
}

// GetHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:read.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceGetHeadlessHostProcedure,
	checkHostPermission(entity.PermKey_HostRead, hostIDFromGet),
)

func (c *ControllerService) GetHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.GetHeadlessHostRequest]) (*connect.Response[hdlctrlv1.GetHeadlessHostResponse], error) {
	host, err := c.hhuc.HeadlessHostGet(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	res := connect.NewResponse(&hdlctrlv1.GetHeadlessHostResponse{
		Host: converter.HeadlessHostEntityToProto(host),
	})

	return res, nil
}

// ListHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: handler 側で resolveListGroupFilter により認可する (interceptor は通過のみ).
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListHeadlessHostProcedure,
	requireAuthOnly,
)

func (c *ControllerService) ListHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.ListHeadlessHostRequest]) (*connect.Response[hdlctrlv1.ListHeadlessHostResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	groupIDs, err := c.resolveListGroupFilter(ctx, req.Msg.GroupId, entity.PermKey_HostRead)
	if err != nil {
		return nil, err
	}

	pageResult, err := c.hhuc.HeadlessHostListPaged(ctx, usecase.HeadlessHostListPagedOptions{
		PageIndex: pageIndex,
		PageSize:  pageSize,
		GroupIDs:  groupIDs,
	})
	if err != nil {
		return nil, convertErr(err)
	}

	protoHosts := make([]*hdlctrlv1.HeadlessHost, 0, len(pageResult.Hosts))
	for _, host := range pageResult.Hosts {
		protoHosts = append(protoHosts, converter.HeadlessHostEntityToProto(host))
	}

	res := connect.NewResponse(&hdlctrlv1.ListHeadlessHostResponse{
		Hosts: protoHosts,
		Page: &hdlctrlv1.PageResponse{
			TotalCount: pageResult.TotalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
		},
	})

	return res, nil
}

// DeleteHeadlessHost implements hdlctrlv1connect.ControllerServiceHandler.
// 権限: host.group_id に対して host:write.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceDeleteHeadlessHostProcedure,
	checkHostPermission(entity.PermKey_HostWrite, hostIDFromDelete),
)

func (c *ControllerService) DeleteHeadlessHost(ctx context.Context, req *connect.Request[hdlctrlv1.DeleteHeadlessHostRequest]) (*connect.Response[hdlctrlv1.DeleteHeadlessHostResponse], error) {
	err := c.hhuc.HeadlessHostDelete(ctx, req.Msg.GetHostId())
	if err != nil {
		return nil, convertErr(err)
	}

	c.publishHostListChanged()

	return connect.NewResponse(&hdlctrlv1.DeleteHeadlessHostResponse{}), nil
}
