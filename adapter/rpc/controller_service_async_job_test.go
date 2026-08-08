package rpc

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultAdminUserID は CreateDefaultAuthenticatedRequest が使うデフォルトユーザー.
// setupControllerServiceTest で system-admin として bootstrap されている.
const defaultAdminUserID = "test@example.test"

func TestControllerService_ListAsyncJobs(t *testing.T) {
	baseTime := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	t.Run("成功: 既定では自分が投入した job のみ返る", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		normalUserID := "normal-user@example.test"
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, normalUserID)

		adminID := defaultAdminUserID
		hostID := "H-own"
		resultPayload := `{"host_id": "H-own"}`

		// 自分の job (完了済み) と、他ユーザーの job を 1 件ずつ用意する.
		ownJobID := testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:       entity.AsyncJobType_START_HOST,
			Status:        entity.AsyncJobStatus_SUCCEEDED,
			CreatedBy:     &normalUserID,
			HostID:        &hostID,
			ResultPayload: &resultPayload,
			CreatedAt:     baseTime,
		})
		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_SHUTDOWN_HOST,
			Status:    entity.AsyncJobStatus_PENDING,
			CreatedBy: &adminID,
			CreatedAt: baseTime.Add(time.Minute),
		})

		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ListAsyncJobsRequest{}, normalUserID, "U-normal", "")

		res, err := client.ListAsyncJobs(t.Context(), req)
		require.NoError(t, err)
		require.Len(t, res.Msg.GetJobs(), 1)

		got := res.Msg.GetJobs()[0]
		assert.Equal(t, ownJobID, got.GetId())
		assert.Equal(t, hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_START_HOST, got.GetJobType())
		assert.Equal(t, hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_SUCCEEDED, got.GetStatus())
		assert.Equal(t, hostID, got.GetHostId())
		assert.Equal(t, normalUserID, got.GetCreatedBy())
		assert.Empty(t, got.GetSessionId())
		assert.Empty(t, got.GetLastError())
		// result_payload は JSON 文字列としてそのまま渡る.
		assert.JSONEq(t, resultPayload, got.GetResultPayload())
		assert.Equal(t, baseTime.Unix(), got.GetCreatedAt().AsTime().Unix())
		assert.Equal(t, baseTime.Unix(), got.GetExecutedAt().AsTime().Unix())

		// total_count も絞り込み後の件数になる.
		assert.Equal(t, int32(1), res.Msg.GetPage().GetTotalCount())
		assert.Equal(t, int32(0), res.Msg.GetPage().GetPageIndex())
		assert.Equal(t, int32(defaultPageSize), res.Msg.GetPage().GetPageSize())
	})

	t.Run("成功: system 権限保持者は include_all_users で全ユーザー分を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		normalUserID := "normal-user@example.test"
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, normalUserID)

		adminID := defaultAdminUserID

		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_START_HOST,
			Status:    entity.AsyncJobStatus_SUCCEEDED,
			CreatedBy: &normalUserID,
			CreatedAt: baseTime,
		})
		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_SHUTDOWN_HOST,
			Status:    entity.AsyncJobStatus_PENDING,
			CreatedBy: &adminID,
			CreatedAt: baseTime.Add(time.Minute),
		})

		includeAll := true
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListAsyncJobsRequest{
			IncludeAllUsers: &includeAll,
		})

		res, err := client.ListAsyncJobs(t.Context(), req)
		require.NoError(t, err)
		require.Len(t, res.Msg.GetJobs(), 2)
		assert.Equal(t, int32(2), res.Msg.GetPage().GetTotalCount())

		// created_at 降順なので、後に作った admin の job が先頭.
		assert.Equal(t, adminID, res.Msg.GetJobs()[0].GetCreatedBy())
		assert.Equal(t, normalUserID, res.Msg.GetJobs()[1].GetCreatedBy())

		// include_all_users を外せば system 権限保持者でも自分の job だけになる.
		ownRes, err := client.ListAsyncJobs(t.Context(),
			testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListAsyncJobsRequest{}))
		require.NoError(t, err)
		require.Len(t, ownRes.Msg.GetJobs(), 1)
		assert.Equal(t, adminID, ownRes.Msg.GetJobs()[0].GetCreatedBy())
	})

	t.Run("失敗: 一般ユーザーが include_all_users を指定すると PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		normalUserID := "normal-user@example.test"
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, normalUserID)

		includeAll := true
		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ListAsyncJobsRequest{
			IncludeAllUsers: &includeAll,
		}, normalUserID, "U-normal", "")

		_, err := client.ListAsyncJobs(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

		// include_all_users なしなら同じユーザーでも通る.
		okRes, err := client.ListAsyncJobs(t.Context(),
			testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ListAsyncJobsRequest{}, normalUserID, "U-normal", ""))
		require.NoError(t, err)
		assert.Empty(t, okRes.Msg.GetJobs())
	})

	t.Run("成功: status / job_type フィルタが効く", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		adminID := defaultAdminUserID

		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_START_HOST,
			Status:    entity.AsyncJobStatus_SUCCEEDED,
			CreatedBy: &adminID,
			CreatedAt: baseTime,
		})
		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_SHUTDOWN_HOST,
			Status:    entity.AsyncJobStatus_FAILED,
			CreatedBy: &adminID,
			CreatedAt: baseTime.Add(time.Minute),
		})
		testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
			JobType:   entity.AsyncJobType_BUILD_IMAGE,
			Status:    entity.AsyncJobStatus_SUCCEEDED,
			CreatedBy: &adminID,
			CreatedAt: baseTime.Add(2 * time.Minute),
		})

		status := hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_SUCCEEDED
		statusRes, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{Status: &status}))
		require.NoError(t, err)
		assert.Len(t, statusRes.Msg.GetJobs(), 2)
		assert.Equal(t, int32(2), statusRes.Msg.GetPage().GetTotalCount())

		jobType := hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_BUILD_IMAGE
		typeRes, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{JobType: &jobType}))
		require.NoError(t, err)
		require.Len(t, typeRes.Msg.GetJobs(), 1)
		assert.Equal(t, hdlctrlv1.AsyncJobType_ASYNC_JOB_TYPE_BUILD_IMAGE, typeRes.Msg.GetJobs()[0].GetJobType())

		// status と job_type の AND.
		pending := hdlctrlv1.AsyncJobStatus_ASYNC_JOB_STATUS_PENDING
		andRes, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{Status: &pending, JobType: &jobType}))
		require.NoError(t, err)
		assert.Empty(t, andRes.Msg.GetJobs())
	})

	t.Run("成功: ページングが created_at 降順で効く", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		adminID := defaultAdminUserID

		ids := make([]string, 0, 5)
		for i := range 5 {
			ids = append(ids, testutil.CreateTestAsyncJob(t, setup.pool, testutil.AsyncJobFixture{
				JobType:   entity.AsyncJobType_START_HOST,
				Status:    entity.AsyncJobStatus_SUCCEEDED,
				CreatedBy: &adminID,
				CreatedAt: baseTime.Add(time.Duration(i) * time.Minute),
			}))
		}

		firstRes, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{Page: &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 2}}))
		require.NoError(t, err)
		require.Len(t, firstRes.Msg.GetJobs(), 2)
		// total_count は page で切る前の全件数.
		assert.Equal(t, int32(5), firstRes.Msg.GetPage().GetTotalCount())
		assert.Equal(t, int32(2), firstRes.Msg.GetPage().GetPageSize())
		assert.Equal(t, ids[4], firstRes.Msg.GetJobs()[0].GetId())
		assert.Equal(t, ids[3], firstRes.Msg.GetJobs()[1].GetId())

		lastRes, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{Page: &hdlctrlv1.PageRequest{PageIndex: 2, PageSize: 2}}))
		require.NoError(t, err)
		require.Len(t, lastRes.Msg.GetJobs(), 1)
		assert.Equal(t, ids[0], lastRes.Msg.GetJobs()[0].GetId())
		assert.Equal(t, int32(2), lastRes.Msg.GetPage().GetPageIndex())
	})

	t.Run("失敗: page_index が負なら InvalidArgument", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		_, err := client.ListAsyncJobs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t,
			&hdlctrlv1.ListAsyncJobsRequest{Page: &hdlctrlv1.PageRequest{PageIndex: -1, PageSize: 10}}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: 未認証なら Unauthenticated", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		_, err := client.ListAsyncJobs(t.Context(), connect.NewRequest(&hdlctrlv1.ListAsyncJobsRequest{}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})
}
