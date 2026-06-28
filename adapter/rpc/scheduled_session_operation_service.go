package rpc

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// 予約操作系 RPC は handler 側 (usecase) で対象 host/session の group_id に対して
// 必要 permission をチェックする (interceptor は通過のみ).
var (
	_ = registerRPCPermission(
		hdlctrlv1connect.ControllerServiceCreateScheduledSessionOperationProcedure,
		requireAuthOnly,
	)
	_ = registerRPCPermission(
		hdlctrlv1connect.ControllerServiceListScheduledSessionOperationsProcedure,
		requireAuthOnly,
	)
	_ = registerRPCPermission(
		hdlctrlv1connect.ControllerServiceCancelScheduledSessionOperationProcedure,
		requireAuthOnly,
	)
)

var headlessUpdateParamsZero = headlessv1.UpdateSessionParametersRequest{}

func startupParamsFromJSON(raw json.RawMessage) *headlessv1.WorldStartupParameters {
	out := &headlessv1.WorldStartupParameters{}
	if len(raw) == 0 {
		return out
	}

	_ = protojson.Unmarshal(raw, out)

	return out
}

func updateParamsFromJSON(raw json.RawMessage) *headlessv1.UpdateSessionParametersRequest {
	if len(raw) == 0 {
		return nil
	}

	out := &headlessv1.UpdateSessionParametersRequest{}
	if err := protojson.Unmarshal(raw, out); err != nil {
		return nil
	}

	return out
}

// CreateScheduledSessionOperation implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) CreateScheduledSessionOperation(ctx context.Context, req *connect.Request[hdlctrlv1.CreateScheduledSessionOperationRequest]) (*connect.Response[hdlctrlv1.CreateScheduledSessionOperationResponse], error) {
	op := req.Msg.GetOperation()
	if op == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("operation is required"))
	}

	trigger := req.Msg.GetTrigger()
	if trigger == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("trigger is required"))
	}

	action, hostID, sessionID, err := buildActionFromProto(op)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	trig, err := buildTriggerFromProto(trigger)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	createdBy := callerUserIDOrNil(ctx)

	created, err := c.souc.Create(ctx, usecase.CreateScheduledSessionOperationParams{
		Action:    action,
		Trigger:   trig,
		HostID:    hostID,
		SessionID: sessionID,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, convertErr(err)
	}

	protoOp, err := scheduledOperationToProto(created)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&hdlctrlv1.CreateScheduledSessionOperationResponse{
		ScheduledOperation: protoOp,
	}), nil
}

// ListScheduledSessionOperations implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListScheduledSessionOperations(ctx context.Context, req *connect.Request[hdlctrlv1.ListScheduledSessionOperationsRequest]) (*connect.Response[hdlctrlv1.ListScheduledSessionOperationsResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	filter := usecase.ListScheduledSessionOperationsFilter{
		PageIndex: pageIndex,
		PageSize:  pageSize,
	}

	if req.Msg.SessionId != nil {
		v := req.Msg.GetSessionId()
		filter.SessionID = &v
	}

	if req.Msg.HostId != nil {
		v := req.Msg.GetHostId()
		filter.HostID = &v
	}

	if req.Msg.Status != nil {
		s := protoStatusToDomain(req.Msg.GetStatus())
		filter.Status = &s
	}

	if req.Msg.GroupId != nil {
		v := req.Msg.GetGroupId()
		filter.GroupID = &v
	}

	result, err := c.souc.List(ctx, filter)
	if err != nil {
		return nil, convertErr(err)
	}

	protoList := make([]*hdlctrlv1.ScheduledSessionOperation, 0, len(result.Items))

	for _, e := range result.Items {
		p, err := scheduledOperationToProto(e)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		protoList = append(protoList, p)
	}

	return connect.NewResponse(&hdlctrlv1.ListScheduledSessionOperationsResponse{
		ScheduledOperations: protoList,
		Page: &hdlctrlv1.PageResponse{
			TotalCount: result.TotalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
		},
	}), nil
}

// CancelScheduledSessionOperation implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) CancelScheduledSessionOperation(ctx context.Context, req *connect.Request[hdlctrlv1.CancelScheduledSessionOperationRequest]) (*connect.Response[hdlctrlv1.CancelScheduledSessionOperationResponse], error) {
	id := req.Msg.GetId()
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	if err := c.souc.Cancel(ctx, id); err != nil {
		if errors.Is(err, usecase.ErrScheduledOperationNotCancelable) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}

		return nil, convertErr(err)
	}

	return connect.NewResponse(&hdlctrlv1.CancelScheduledSessionOperationResponse{}), nil
}

// buildActionFromProto は ScheduledOperation oneof → scheduled_op.Action 変換と、
// 一覧フィルタ用の (host_id, session_id) の抽出を兼ねる.
func buildActionFromProto(op *hdlctrlv1.ScheduledOperation) (scheduled_op.Action, *string, *string, error) {
	switch x := op.GetOperation().(type) {
	case *hdlctrlv1.ScheduledOperation_StartSession:
		start := x.StartSession
		if start.GetHostId() == "" {
			return nil, nil, nil, errors.New("start_session: host_id is required")
		}

		paramsJSON, err := protojson.Marshal(start.GetParameters())
		if err != nil {
			return nil, nil, nil, err
		}

		var memo *string

		if start.GetMemo() != "" {
			m := start.GetMemo()
			memo = &m
		}

		hostID := start.GetHostId()
		act := actions.NewStartSessionAction(hostID, start.GetGroupId(), nil, memo, paramsJSON)

		return act, &hostID, nil, nil
	case *hdlctrlv1.ScheduledOperation_StopSession:
		stop := x.StopSession
		if stop.GetSessionId() == "" {
			return nil, nil, nil, errors.New("stop_session: session_id is required")
		}

		sid := stop.GetSessionId()
		act := actions.NewStopSessionAction(sid)

		return act, nil, &sid, nil
	case *hdlctrlv1.ScheduledOperation_UpdateParameters:
		upd := x.UpdateParameters

		inner := upd.GetParameters()
		if inner == nil {
			return nil, nil, nil, errors.New("update_parameters: parameters is required")
		}

		sid := inner.GetSessionId()
		if sid == "" {
			return nil, nil, nil, errors.New("update_parameters: parameters.session_id is required")
		}

		paramsJSON, err := protojson.Marshal(inner)
		if err != nil {
			return nil, nil, nil, err
		}

		act := actions.NewUpdateParametersAction(sid, paramsJSON)

		return act, nil, &sid, nil
	case *hdlctrlv1.ScheduledOperation_UpdateExtraSettings:
		upd := x.UpdateExtraSettings

		sid := upd.GetSessionId()
		if sid == "" {
			return nil, nil, nil, errors.New("update_extra_settings: session_id is required")
		}

		var autoUpgrade *bool

		if upd.AutoUpgrade != nil {
			v := upd.GetAutoUpgrade()
			autoUpgrade = &v
		}

		var memo *string

		if upd.Memo != nil {
			v := upd.GetMemo()
			memo = &v
		}

		act := actions.NewUpdateExtraSettingsAction(sid, autoUpgrade, memo)

		return act, nil, &sid, nil
	default:
		return nil, nil, nil, errors.New("operation oneof is not set")
	}
}

func buildTriggerFromProto(tr *hdlctrlv1.ScheduledTrigger) (scheduled_op.Trigger, error) {
	switch x := tr.GetTrigger().(type) {
	case *hdlctrlv1.ScheduledTrigger_Time:
		ts := x.Time.GetScheduledAt()
		if ts == nil {
			return nil, errors.New("time trigger: scheduled_at is required")
		}

		return triggers.NewTimeTrigger(ts.AsTime()), nil
	case *hdlctrlv1.ScheduledTrigger_SessionUserCount:
		c := x.SessionUserCount
		sid := c.GetSessionId()

		if sid == "" {
			return nil, errors.New("session_user_count trigger: session_id is required")
		}

		var cmp triggers.SessionUserCountComparator

		switch c.GetComparator() {
		case hdlctrlv1.SessionUserCountTrigger_COMPARATOR_LESS_OR_EQUAL:
			cmp = triggers.SessionUserCountComparator_LESS_OR_EQUAL
		case hdlctrlv1.SessionUserCountTrigger_COMPARATOR_GREATER_OR_EQUAL:
			cmp = triggers.SessionUserCountComparator_GREATER_OR_EQUAL
		case hdlctrlv1.SessionUserCountTrigger_COMPARATOR_UNSPECIFIED:
			return nil, errors.New("session_user_count trigger: comparator is required")
		default:
			return nil, errors.New("session_user_count trigger: unknown comparator")
		}

		threshold := c.GetThreshold()
		if threshold < 0 {
			return nil, errors.New("session_user_count trigger: threshold must be >= 0")
		}

		return triggers.NewSessionUserCountTrigger(sid, cmp, threshold), nil
	default:
		return nil, errors.New("trigger oneof is not set")
	}
}

func scheduledOperationToProto(e *entity.ScheduledSessionOperation) (*hdlctrlv1.ScheduledSessionOperation, error) {
	out := &hdlctrlv1.ScheduledSessionOperation{
		Id:         e.ID,
		NextFireAt: timestamppb.New(e.NextFireAt),
		Status:     domainStatusToProto(e.Status),
		CreatedAt:  timestamppb.New(e.CreatedAt),
		UpdatedAt:  timestamppb.New(e.UpdatedAt),
	}

	if e.HostID != nil {
		v := *e.HostID
		out.HostId = &v
	}

	if e.SessionID != nil {
		v := *e.SessionID
		out.SessionId = &v
	}

	if e.LastError != nil {
		v := *e.LastError
		out.LastError = &v
	}

	if e.ExecutedAt != nil {
		out.ExecutedAt = timestamppb.New(*e.ExecutedAt)
	}

	if e.CreatedBy != nil {
		v := *e.CreatedBy
		out.CreatedBy = &v
	}

	// trigger & operation を decode → proto に戻す.
	trig, err := scheduled_op.DecodeTrigger(e.TriggerType, e.TriggerConfig)
	if err != nil {
		return nil, err
	}

	protoTrig, err := triggerToProto(trig)
	if err != nil {
		return nil, err
	}

	out.Trigger = protoTrig

	act, err := scheduled_op.DecodeAction(e.OperationType, e.OperationPayload)
	if err != nil {
		return nil, err
	}

	protoAct, err := actionToProto(act)
	if err != nil {
		return nil, err
	}

	out.Operation = protoAct

	return out, nil
}

func triggerToProto(t scheduled_op.Trigger) (*hdlctrlv1.ScheduledTrigger, error) {
	switch v := t.(type) {
	case *triggers.TimeTrigger:
		return &hdlctrlv1.ScheduledTrigger{
			Trigger: &hdlctrlv1.ScheduledTrigger_Time{
				Time: &hdlctrlv1.TimeTrigger{
					ScheduledAt: timestamppb.New(v.ScheduledAt),
				},
			},
		}, nil
	case *triggers.SessionUserCountTrigger:
		var cmp hdlctrlv1.SessionUserCountTrigger_Comparator

		switch v.Comparator {
		case triggers.SessionUserCountComparator_LESS_OR_EQUAL:
			cmp = hdlctrlv1.SessionUserCountTrigger_COMPARATOR_LESS_OR_EQUAL
		case triggers.SessionUserCountComparator_GREATER_OR_EQUAL:
			cmp = hdlctrlv1.SessionUserCountTrigger_COMPARATOR_GREATER_OR_EQUAL
		}

		return &hdlctrlv1.ScheduledTrigger{
			Trigger: &hdlctrlv1.ScheduledTrigger_SessionUserCount{
				SessionUserCount: &hdlctrlv1.SessionUserCountTrigger{
					SessionId:  v.SessionID,
					Comparator: cmp,
					Threshold:  v.Threshold,
				},
			},
		}, nil
	default:
		return nil, errors.New("unknown trigger type")
	}
}

func actionToProto(a scheduled_op.Action) (*hdlctrlv1.ScheduledOperation, error) {
	switch v := a.(type) {
	case *actions.StartSessionAction:
		params := &hdlctrlv1.StartWorldRequest{HostId: v.HostID}
		if len(v.StartupParamsJSON) > 0 {
			// 復元失敗しても表示用なので fatal にしない.
			startupParams := startupParamsFromJSON(v.StartupParamsJSON)
			params.Parameters = startupParams
		}

		if v.Memo != nil {
			params.Memo = *v.Memo
		}

		return &hdlctrlv1.ScheduledOperation{
			Operation: &hdlctrlv1.ScheduledOperation_StartSession{StartSession: params},
		}, nil
	case *actions.StopSessionAction:
		return &hdlctrlv1.ScheduledOperation{
			Operation: &hdlctrlv1.ScheduledOperation_StopSession{
				StopSession: &hdlctrlv1.StopSessionRequest{SessionId: v.SessionID},
			},
		}, nil
	case *actions.UpdateParametersAction:
		inner := updateParamsFromJSON(v.ParamsJSON)
		if inner == nil {
			inner = &headlessUpdateParamsZero
		}

		inner.SessionId = v.SessionID

		return &hdlctrlv1.ScheduledOperation{
			Operation: &hdlctrlv1.ScheduledOperation_UpdateParameters{
				UpdateParameters: &hdlctrlv1.UpdateSessionParametersRequest{Parameters: inner},
			},
		}, nil
	case *actions.UpdateExtraSettingsAction:
		req := &hdlctrlv1.UpdateSessionExtraSettingsRequest{SessionId: v.SessionID}
		if v.AutoUpgrade != nil {
			req.AutoUpgrade = v.AutoUpgrade
		}

		if v.Memo != nil {
			req.Memo = v.Memo
		}

		return &hdlctrlv1.ScheduledOperation{
			Operation: &hdlctrlv1.ScheduledOperation_UpdateExtraSettings{UpdateExtraSettings: req},
		}, nil
	default:
		return nil, errors.New("unknown action type")
	}
}

func protoStatusToDomain(p hdlctrlv1.ScheduledOperationStatus) entity.ScheduledOperationStatus {
	switch p {
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_PENDING:
		return entity.ScheduledOperationStatus_PENDING
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_RUNNING:
		return entity.ScheduledOperationStatus_RUNNING
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_SUCCEEDED:
		return entity.ScheduledOperationStatus_SUCCEEDED
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_FAILED:
		return entity.ScheduledOperationStatus_FAILED
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_CANCELED:
		return entity.ScheduledOperationStatus_CANCELED
	case hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_UNSPECIFIED:
		return entity.ScheduledOperationStatus_PENDING
	default:
		return entity.ScheduledOperationStatus_PENDING
	}
}

func domainStatusToProto(s entity.ScheduledOperationStatus) hdlctrlv1.ScheduledOperationStatus {
	switch s {
	case entity.ScheduledOperationStatus_PENDING:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_PENDING
	case entity.ScheduledOperationStatus_RUNNING:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_RUNNING
	case entity.ScheduledOperationStatus_SUCCEEDED:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_SUCCEEDED
	case entity.ScheduledOperationStatus_FAILED:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_FAILED
	case entity.ScheduledOperationStatus_CANCELED:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_CANCELED
	default:
		return hdlctrlv1.ScheduledOperationStatus_SCHEDULED_OPERATION_STATUS_UNSPECIFIED
	}
}

// callerUserIDOrNil は AuthInterceptor が ctx に詰めた user id を取り出す.
// 失敗時は CLI / system 由来とみなして nil を返す.
func callerUserIDOrNil(ctx context.Context) *string {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil || claims == nil || claims.UserID == "" {
		return nil
	}

	uid := claims.UserID

	return &uid
}
