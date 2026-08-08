package rpc

import (
	"context"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/async_job"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// job 履歴は「自分の job なら誰でも / 他ユーザー分は system 権限」という
// リクエスト内容依存の認可なので、interceptor は通過のみとし usecase 側で判定する.
var _ = registerRPCPermission(
	hdlctrlv1connect.ControllerServiceListAsyncJobsProcedure,
	requireAuthOnly,
)

// ListAsyncJobs implements hdlctrlv1connect.ControllerServiceHandler.
func (c *ControllerService) ListAsyncJobs(ctx context.Context, req *connect.Request[hdlctrlv1.ListAsyncJobsRequest]) (*connect.Response[hdlctrlv1.ListAsyncJobsResponse], error) {
	pageIndex, pageSize, err := normalizePageRequest(req.Msg.GetPage())
	if err != nil {
		return nil, err
	}

	filter := async_job.ListFilter{
		IncludeAllUsers: req.Msg.GetIncludeAllUsers(),
		PageIndex:       pageIndex,
		PageSize:        pageSize,
	}

	// 明示的に UNSPECIFIED が来た場合は「絞り込みなし」として扱う.
	// 変換すると status は PENDING、job_type は UNKNOWN になり、クライアントの
	// 意図しない絞り込み (job_type は常に 0 件) になってしまうため.
	if req.Msg.Status != nil && req.Msg.GetStatus() != hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_UNSPECIFIED {
		s := protoAsyncJobStatusToDomain(req.Msg.GetStatus())
		filter.Status = &s
	}

	if req.Msg.JobType != nil && req.Msg.GetJobType() != hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_UNSPECIFIED {
		jt := protoAsyncJobTypeToDomain(req.Msg.GetJobType())
		filter.JobType = &jt
	}

	result, err := c.ajuc.List(ctx, filter)
	if err != nil {
		return nil, convertErr(err)
	}

	jobs := make([]*hdlctrlv1.AsyncJob, 0, len(result.Items))
	for _, e := range result.Items {
		jobs = append(jobs, asyncJobToProto(e))
	}

	return connect.NewResponse(&hdlctrlv1.ListAsyncJobsResponse{
		Jobs: jobs,
		Page: &hdlctrlv1.PageResponse{
			TotalCount: result.TotalCount,
			PageIndex:  pageIndex,
			PageSize:   pageSize,
		},
	}), nil
}

func asyncJobToProto(e *entity.AsyncJob) *hdlctrlv1.AsyncJob {
	out := &hdlctrlv1.AsyncJob{
		Id:        e.ID,
		JobType:   domainAsyncJobTypeToProto(e.JobType),
		Status:    domainAsyncJobStatusToProto(e.Status),
		CreatedAt: timestamppb.New(e.CreatedAt),
		UpdatedAt: timestamppb.New(e.UpdatedAt),
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

	// result_payload は未完了なら SQL NULL、MarkSucceeded で結果が無い場合は JSON null が入る.
	// どちらもクライアントには「無し」として見せる.
	if len(e.ResultPayload) > 0 && string(e.ResultPayload) != "null" {
		v := string(e.ResultPayload)
		out.ResultPayload = &v
	}

	if e.CreatedBy != nil {
		v := *e.CreatedBy
		out.CreatedBy = &v
	}

	if e.ExecutedAt != nil {
		out.ExecutedAt = timestamppb.New(*e.ExecutedAt)
	}

	return out
}

func protoAsyncJobStatusToDomain(p hdlctrlv1.AsyncJobStatus) entity.AsyncJobStatus {
	switch p {
	case hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_PENDING:
		return entity.AsyncJobStatus_PENDING
	case hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_RUNNING:
		return entity.AsyncJobStatus_RUNNING
	case hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_SUCCEEDED:
		return entity.AsyncJobStatus_SUCCEEDED
	case hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_FAILED:
		return entity.AsyncJobStatus_FAILED
	case hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_UNSPECIFIED:
		return entity.AsyncJobStatus_PENDING
	default:
		return entity.AsyncJobStatus_PENDING
	}
}

func domainAsyncJobStatusToProto(s entity.AsyncJobStatus) hdlctrlv1.AsyncJobStatus {
	switch s {
	case entity.AsyncJobStatus_PENDING:
		return hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_PENDING
	case entity.AsyncJobStatus_RUNNING:
		return hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_RUNNING
	case entity.AsyncJobStatus_SUCCEEDED:
		return hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_SUCCEEDED
	case entity.AsyncJobStatus_FAILED:
		return hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_FAILED
	default:
		return hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_UNSPECIFIED
	}
}

func protoAsyncJobTypeToDomain(p hdlctrlv1.AsyncJobType) entity.AsyncJobType {
	switch p {
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_START_HOST:
		return entity.AsyncJobType_START_HOST
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_SHUTDOWN_HOST:
		return entity.AsyncJobType_SHUTDOWN_HOST
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_RESTART_HOST:
		return entity.AsyncJobType_RESTART_HOST
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_START_SESSION:
		return entity.AsyncJobType_START_SESSION
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_STOP_SESSION:
		return entity.AsyncJobType_STOP_SESSION
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_BUILD_IMAGE:
		return entity.AsyncJobType_BUILD_IMAGE
	case hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_UNSPECIFIED:
		return entity.AsyncJobType_UNKNOWN
	default:
		return entity.AsyncJobType_UNKNOWN
	}
}

func domainAsyncJobTypeToProto(t entity.AsyncJobType) hdlctrlv1.AsyncJobType {
	switch t {
	case entity.AsyncJobType_START_HOST:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_START_HOST
	case entity.AsyncJobType_SHUTDOWN_HOST:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_SHUTDOWN_HOST
	case entity.AsyncJobType_RESTART_HOST:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_RESTART_HOST
	case entity.AsyncJobType_START_SESSION:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_START_SESSION
	case entity.AsyncJobType_STOP_SESSION:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_STOP_SESSION
	case entity.AsyncJobType_BUILD_IMAGE:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_BUILD_IMAGE
	case entity.AsyncJobType_UNKNOWN:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_UNSPECIFIED
	default:
		return hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_UNSPECIFIED
	}
}
