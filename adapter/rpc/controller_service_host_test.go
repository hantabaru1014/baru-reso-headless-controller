package rpc

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_ListHeadlessHostImageTags(t *testing.T) {
	t.Run("成功: イメージタグ一覧を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Mock HostConnector to return test tags
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), nil).
			Return(port.ContainerImageList{
				{
					Tag:             "2024.1.1-v1.0.0",
					ResoniteVersion: "2024.1.1",
					IsPreRelease:    false,
					AppVersion:      "v1.0.0",
				},
				{
					Tag:             "prerelease-2024.1.2-v1.1.0",
					ResoniteVersion: "2024.1.2",
					IsPreRelease:    true,
					AppVersion:      "v1.1.0",
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		res, err := client.ListHeadlessHostImageTags(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetTags(), 2)

		// Verify first tag
		assert.Equal(t, "2024.1.1-v1.0.0", res.Msg.GetTags()[0].GetTag())
		assert.Equal(t, "2024.1.1", res.Msg.GetTags()[0].GetResoniteVersion())
		assert.False(t, res.Msg.GetTags()[0].GetIsPrerelease())
		assert.Equal(t, "v1.0.0", res.Msg.GetTags()[0].GetAppVersion())

		// Verify second tag
		assert.Equal(t, "prerelease-2024.1.2-v1.1.0", res.Msg.GetTags()[1].GetTag())
		assert.Equal(t, "2024.1.2", res.Msg.GetTags()[1].GetResoniteVersion())
		assert.True(t, res.Msg.GetTags()[1].GetIsPrerelease())
		assert.Equal(t, "v1.1.0", res.Msg.GetTags()[1].GetAppVersion())
	})

	t.Run("失敗: コネクタでエラー発生", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Mock HostConnector to return error
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), nil).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		_, err := client.ListHeadlessHostImageTags(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StartHeadlessHost(t *testing.T) {
	t.Run("成功: 非同期 job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")

		imageTag := "latest"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-test",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		})

		res, err := client.StartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_START_HOST))
	})

	t.Run("成功: 最小権限 caller (host:write + account:use) で起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const callerID = "U-mp-starthost"

		const groupID = "g-mp-starthost"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)

		imageTag := "latest"
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-mp-acc",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		}, callerID, groupID, []string{
			entity.PermKey_HostWrite,
			entity.PermKey_AccountUse,
		})

		res, err := client.StartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_START_HOST))
	})

	t.Run("失敗: host:write のみ (account:use 不足) で PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const callerID = "U-mp-starthost-noaccuse"

		const groupID = "g-mp-starthost-noaccuse"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)

		imageTag := "latest"
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-mp-acc",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		}, callerID, groupID, []string{entity.PermKey_HostWrite})

		_, err := client.StartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_AccountUse)
	})

	t.Run("失敗: account:use のみ (host:write 不足) で PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const callerID = "U-mp-starthost-nohostwrite"

		const groupID = "g-mp-starthost-nohostwrite"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)

		imageTag := "latest"
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-mp-acc",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		}, callerID, groupID, []string{entity.PermKey_AccountUse})

		_, err := client.StartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_HostWrite)
	})

	// 権限システム導入後は account の所属グループに対する permission を確認するため、
	// 存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		imageTag := "latest"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-nonexist",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		})

		_, err := client.StartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

// assertJobEnqueued は jobId に対応する async_jobs 行が PENDING で
// 期待した job_type を持つことを確認する.
func assertJobEnqueued(t *testing.T, setup *controllerServiceTestSetup, jobID string, expectedType int32) {
	t.Helper()

	require.NotEmpty(t, jobID, "job_id should be returned")

	var uid pgtype.UUID
	require.NoError(t, uid.Scan(jobID))

	row, err := setup.queries.GetAsyncJob(t.Context(), uid)
	require.NoError(t, err, "enqueued job should exist in async_jobs table")
	assert.Equal(t, expectedType, row.JobType)
	assert.Equal(t, int32(entity.AsyncJobStatus_PENDING), row.Status)
}

func TestControllerService_RestartHeadlessHost(t *testing.T) {
	t.Run("成功: 非同期 job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// 受付時 host 存在確認は EXITED で十分 (RUNNING にすると dbToEntity が
		// GetRpcClient を引いてしまい、RPC 副作用の伴わない unit test にできない).
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.RestartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_RESTART_HOST))
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-restart"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-restart-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-restart-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: host.ID,
		}, "U-mp-restart", groupID, []string{entity.PermKey_HostWrite})

		res, err := client.RestartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_RESTART_HOST))
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: "nonexist-host",
		})

		_, err := client.RestartHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_UpdateHeadlessHostSettings(t *testing.T) {
	t.Run("成功: 停止中のホストの設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		newName := "UpdatedHost"
		newTickRate := float32(120)
		newPolicy := hdlctrlv1.HeadlessHostAutoUpdatePolicy_HEADLESS_HOST_AUTO_UPDATE_POLICY_USERS_EMPTY

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId:           host.ID,
			Name:             &newName,
			TickRate:         &newTickRate,
			AutoUpdatePolicy: &newPolicy,
		})

		res, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was updated in database
		updatedHost, err := setup.queries.GetHost(t.Context(), host.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, updatedHost.Name)
		assert.Equal(t, int32(entity.HostAutoUpdatePolicy_USERS_EMPTY), updatedHost.AutoUpdatePolicy,
			"AutoUpdatePolicy passed in request should be persisted")
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-updsettings"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-upd-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-upd-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		newName := "UpdatedHostByMinPerm"
		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId: host.ID,
			Name:   &newName,
		}, "U-mp-updsettings", groupID, []string{entity.PermKey_HostWrite})

		res, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: 実行中のホストの設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and running host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "RunningHost", entity.HeadlessHostStatus_RUNNING)

		// Mock RPC calls - GetRpcClient is called twice: once in dbToEntity, once before UpdateHostSettings
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			Times(2)

		newTickRate := float32(120)
		maxTransfers := int32(4)

		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{}, nil)

		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{}, nil)

		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{}, nil)

		// GetStartupConfigToRestore is called twice: once in dbToEntity, once after UpdateHostSettings
		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{
					TickRate:                    &newTickRate,
					MaxConcurrentAssetTransfers: &maxTransfers,
				},
			}, nil).
			Times(2)

		setup.mockRpcClient.EXPECT().
			UpdateHostSettings(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateHostSettingsResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId:   host.ID,
			TickRate: &newTickRate,
		})

		res, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateHeadlessHostSettingsRequest{
			HostId: "nonexist",
		})

		_, err := client.UpdateHeadlessHostSettings(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessHostLogs(t *testing.T) {
	t.Run("成功: ログを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		// Insert test logs into container_logs table
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime, "stdout", "Log line 1")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Second), "stderr", "Error line")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetLogs(), 2)
		assert.Equal(t, "Log line 1", res.Msg.GetLogs()[0].GetBody())
		assert.False(t, res.Msg.GetLogs()[0].GetIsError())
		assert.Equal(t, "Error line", res.Msg.GetLogs()[1].GetBody())
		assert.True(t, res.Msg.GetLogs()[1].GetIsError())
	})

	t.Run("成功: 最小権限 caller (host:read) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-getlogs"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-getlogs-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-getlogs-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime, "stdout", "Log A")

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		}, "U-mp-getlogs", groupID, []string{entity.PermKey_HostRead})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetLogs(), 1)
	})

	t.Run("成功: ログが存在しない場合は空のリストを返す", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host (no logs inserted)
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Empty(t, res.Msg.GetLogs())
	})

	t.Run("成功: limitパラメータでログ件数を制限", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_EXITED)

		// Insert 5 logs
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		for i := range 5 {
			testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Duration(i)*time.Second), "stdout", fmt.Sprintf("Log line %d", i+1))
		}

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      3,
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetLogs(), 3)
		assert.True(t, res.Msg.GetHasMoreBefore(), "should have more logs before")
		assert.False(t, res.Msg.GetHasMoreAfter(), "should not have more logs after (initial fetch)")
	})

	t.Run("成功: beforeIdカーソルで古いログを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test4", "test4@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test4", "TestHost4", entity.HeadlessHostStatus_EXITED)

		// Insert logs with distinct timestamps
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime, "stdout", "Old log")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Minute), "stdout", "Middle log")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(2*time.Minute), "stdout", "New log")

		// First, get all logs to find the ID of "Middle log"
		allLogsReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})
		allLogsRes, err := client.GetHeadlessHostLogs(t.Context(), allLogsReq)
		require.NoError(t, err)
		require.Len(t, allLogsRes.Msg.GetLogs(), 3)

		// Find the middle log's ID (logs are returned in chronological order: Old, Middle, New)
		var middleLogId int64

		for _, log := range allLogsRes.Msg.GetLogs() {
			if log.GetBody() == "Middle log" {
				middleLogId = log.GetId()

				break
			}
		}

		require.NotZero(t, middleLogId, "should find middle log")

		// Use beforeId cursor to get logs before "Middle log"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_BeforeId{BeforeId: middleLogId},
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetLogs(), 1)
		assert.Equal(t, "Old log", res.Msg.GetLogs()[0].GetBody())
		assert.False(t, res.Msg.GetHasMoreBefore(), "no more older logs")
	})

	t.Run("成功: afterIdカーソルで新しいログを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test5", "test5@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test5", "TestHost5", entity.HeadlessHostStatus_EXITED)

		// Insert logs with distinct timestamps
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime, "stdout", "Old log")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Minute), "stdout", "Middle log")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(2*time.Minute), "stdout", "New log")

		// First, get all logs to find the ID of "Middle log"
		allLogsReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})
		allLogsRes, err := client.GetHeadlessHostLogs(t.Context(), allLogsReq)
		require.NoError(t, err)
		require.Len(t, allLogsRes.Msg.GetLogs(), 3)

		// Find the middle log's ID (logs are returned in chronological order: Old, Middle, New)
		var middleLogId int64

		for _, log := range allLogsRes.Msg.GetLogs() {
			if log.GetBody() == "Middle log" {
				middleLogId = log.GetId()

				break
			}
		}

		require.NotZero(t, middleLogId, "should find middle log")

		// Use afterId cursor to get logs after "Middle log"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AfterId{AfterId: middleLogId},
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetLogs(), 1)
		assert.Equal(t, "New log", res.Msg.GetLogs()[0].GetBody())
		assert.False(t, res.Msg.GetHasMoreAfter(), "no more newer logs")
	})

	t.Run("成功: 異なるinstanceIdでログを分離", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test6", "test6@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test6", "TestHost6", entity.HeadlessHostStatus_EXITED)

		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		// Insert logs for instance 1 (current)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime, "stdout", "Current instance log")
		// Insert logs for instance 0 (previous)
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, 0, baseTime, "stdout", "Previous instance log")

		// Request current instance logs
		req1 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})
		res1, err := client.GetHeadlessHostLogs(t.Context(), req1)
		require.NoError(t, err)
		assert.Len(t, res1.Msg.GetLogs(), 1)
		assert.Equal(t, "Current instance log", res1.Msg.GetLogs()[0].GetBody())

		// Request previous instance logs
		req2 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: 0,
		})
		res2, err := client.GetHeadlessHostLogs(t.Context(), req2)
		require.NoError(t, err)
		assert.Len(t, res2.Msg.GetLogs(), 1)
		assert.Equal(t, "Previous instance log", res2.Msg.GetLogs()[0].GetBody())
	})

	t.Run("成功: has_more_afterフラグが正しく設定される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test7", "test7@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test7", "TestHost7", entity.HeadlessHostStatus_EXITED)

		// Insert 5 logs
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		for i := range 5 {
			testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Duration(i)*time.Minute), "stdout", fmt.Sprintf("Log %d", i+1))
		}

		// First, get all logs to find the ID of "Log 1" (oldest)
		allLogsReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
		})
		allLogsRes, err := client.GetHeadlessHostLogs(t.Context(), allLogsReq)
		require.NoError(t, err)
		require.Len(t, allLogsRes.Msg.GetLogs(), 5)

		// Get the first log's ID (oldest, "Log 1")
		firstLogId := allLogsRes.Msg.GetLogs()[0].GetId()
		require.NotZero(t, firstLogId, "should have first log ID")

		// Use afterId cursor with limit to trigger has_more_after
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      2,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AfterId{AfterId: firstLogId},
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetLogs(), 2)
		assert.True(t, res.Msg.GetHasMoreAfter(), "should have more logs after")
		assert.False(t, res.Msg.GetHasMoreBefore(), "should not have has_more_before with after cursor")
	})
}

func TestControllerService_ShutdownHeadlessHost(t *testing.T) {
	t.Run("成功: 非同期 job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_SHUTDOWN_HOST))
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-shutdown"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-shut-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-shut-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: host.ID,
		}, "U-mp-shutdown", groupID, []string{entity.PermKey_HostWrite})

		res, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_SHUTDOWN_HOST))
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_KillHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを強制停止", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - Kill
		setup.mockHostConnector.EXPECT().
			Kill(gomock.Any(), gomock.Any()).
			Return(nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KillHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.KillHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-kill"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-kill-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-kill-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().Kill(gomock.Any(), gomock.Any()).Return(nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.KillHeadlessHostRequest{
			HostId: host.ID,
		}, "U-mp-kill", groupID, []string{entity.PermKey_HostWrite})

		_, err := client.KillHeadlessHost(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KillHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.KillHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_GetHeadlessHost(t *testing.T) {
	t.Run("成功: ホストの詳細を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.GetHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotNil(t, res.Msg.GetHost())
		assert.Equal(t, host.ID, res.Msg.GetHost().GetId())
		assert.Equal(t, "TestHost", res.Msg.GetHost().GetName())
	})

	t.Run("成功: 最小権限 caller (host:read) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-gethost"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-gethost-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-gethost-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: host.ID,
		}, "U-mp-gethost", groupID, []string{entity.PermKey_HostRead})

		res, err := client.GetHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, host.ID, res.Msg.GetHost().GetId())
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.GetHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_ListHeadlessHost(t *testing.T) {
	t.Run("成功: ホスト一覧を取得 (ページング検証 / system:group.list 保持者は全件)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create 5 test hosts for pagination
		const totalHosts = 5

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")

		for i := 1; i <= totalHosts; i++ {
			testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", fmt.Sprintf("Host%d", i), entity.HeadlessHostStatus_EXITED)
		}

		// page 未指定 -> デフォルト 20 件 (5件しかないので全件返る)
		// 既定ユーザーは system-admin (system:group.list 保持) なので
		// group_id 未指定でも全グループのホストを返す.
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{})
		res, err := client.ListHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetHosts(), totalHosts)
		require.NotNil(t, res.Msg.GetPage())
		assert.Equal(t, int32(totalHosts), res.Msg.GetPage().GetTotalCount())
		assert.Equal(t, int32(20), res.Msg.GetPage().GetPageSize())

		// page_size=3 で 1 ページ目 -> 3 件
		req2 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 3},
		})
		res2, err := client.ListHeadlessHost(t.Context(), req2)
		require.NoError(t, err)
		assert.Len(t, res2.Msg.GetHosts(), 3)
		assert.Equal(t, int32(totalHosts), res2.Msg.GetPage().GetTotalCount())

		// page_size=3, page_index=1 -> 残り 2 件
		req3 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: 1, PageSize: 3},
		})
		res3, err := client.ListHeadlessHost(t.Context(), req3)
		require.NoError(t, err)
		assert.Len(t, res3.Msg.GetHosts(), 2)
		assert.Equal(t, int32(totalHosts), res3.Msg.GetPage().GetTotalCount())

		// page_index=-1 -> CodeInvalidArgument
		req4 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{
			Page: &hdlctrlv1.PageRequest{PageIndex: -1, PageSize: 20},
		})
		_, err = client.ListHeadlessHost(t.Context(), req4)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("グループフィルタ: 指定 / 未指定 / 権限なしの分岐", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// non-admin ユーザーを personal グループ単独所属で用意し、別 normal グループを
		// 作って admin として追加. 既存の "migrated-pre-permission" には所属させない.
		const otherUserID = "U-other"

		personalGID := testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, otherUserID)

		sharedGID := "group-shared"
		testutil.CreateTestGroup(t, setup.queries, sharedGID, otherUserID)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		// hosts: migrated グループに 2 件, personal に 1 件, shared に 1 件.
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "MigratedHost1", entity.HeadlessHostStatus_EXITED)
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "MigratedHost2", entity.HeadlessHostStatus_EXITED)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-test", "PersonalHost", entity.HeadlessHostStatus_EXITED, personalGID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-test", "SharedHost", entity.HeadlessHostStatus_EXITED, sharedGID)

		// case 1: non-admin ユーザー / group_id 未指定 -> 自分が所属する personal + shared のみ.
		reqAuto := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessHostRequest{},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		resAuto, err := client.ListHeadlessHost(t.Context(), reqAuto)
		require.NoError(t, err)
		assert.Equal(t, int32(2), resAuto.Msg.GetPage().GetTotalCount(),
			"non-admin should only see hosts in groups they belong to")

		gotNames := make(map[string]bool, len(resAuto.Msg.GetHosts()))
		for _, h := range resAuto.Msg.GetHosts() {
			gotNames[h.GetName()] = true
		}

		assert.True(t, gotNames["PersonalHost"], "expected PersonalHost in result")
		assert.True(t, gotNames["SharedHost"], "expected SharedHost in result")
		assert.False(t, gotNames["MigratedHost1"], "MigratedHost1 should be filtered out")

		// case 2: non-admin ユーザー / 自分が所属する group_id を指定 -> その group のみ.
		reqExplicit := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessHostRequest{GroupId: &sharedGID},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		resExplicit, err := client.ListHeadlessHost(t.Context(), reqExplicit)
		require.NoError(t, err)
		assert.Equal(t, int32(1), resExplicit.Msg.GetPage().GetTotalCount())
		require.Len(t, resExplicit.Msg.GetHosts(), 1)
		assert.Equal(t, "SharedHost", resExplicit.Msg.GetHosts()[0].GetName())

		// case 3: non-admin ユーザー / 権限の無い group_id を指定 -> PermissionDenied.
		forbiddenGID := entity.MigratedPrePermissionGroupID

		reqForbidden := testutil.CreateAuthenticatedRequest(
			t, &hdlctrlv1.ListHeadlessHostRequest{GroupId: &forbiddenGID},
			otherUserID, "U-other-resonite", "https://example.test/icon.png",
		)
		_, err = client.ListHeadlessHost(t.Context(), reqForbidden)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

		// case 4: 既定ユーザー (system-admin) が同じ group_id を指定 -> system:group.list 経由で許可.
		reqAdmin := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostRequest{
			GroupId: &sharedGID,
		})
		resAdmin, err := client.ListHeadlessHost(t.Context(), reqAdmin)
		require.NoError(t, err)
		assert.Equal(t, int32(1), resAdmin.Msg.GetPage().GetTotalCount())
	})
}

func TestControllerService_DeleteHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		// Add container logs for this host (multiple instances)
		now := time.Now()
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, 1, now, "stdout", "log message 1")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, 1, now.Add(time.Second), "stderr", "error message")
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, 2, now, "stdout", "log from instance 2")

		// Verify logs exist before deletion
		logsBefore, err := setup.queries.GetContainerLogsByTag(t.Context(), db.GetContainerLogsByTagParams{
			Tag:     pgtype.Text{String: "headless-" + host.ID + "-1", Valid: true},
			MaxRows: int32(100),
		})
		require.NoError(t, err)
		assert.Len(t, logsBefore, 2, "should have 2 logs for instance 1 before deletion")

		// Mock the container removal
		setup.mockHostConnector.EXPECT().
			Remove(gomock.Any(), gomock.Any()).
			Return(nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.DeleteHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was deleted from database
		_, err = setup.queries.GetHost(t.Context(), host.ID)
		require.Error(t, err)

		// Verify container logs were also deleted
		logsAfter1, err := setup.queries.GetContainerLogsByTag(t.Context(), db.GetContainerLogsByTagParams{
			Tag:     pgtype.Text{String: "headless-" + host.ID + "-1", Valid: true},
			MaxRows: int32(100),
		})
		require.NoError(t, err)
		assert.Empty(t, logsAfter1, "logs for instance 1 should be deleted")

		logsAfter2, err := setup.queries.GetContainerLogsByTag(t.Context(), db.GetContainerLogsByTagParams{
			Tag:     pgtype.Text{String: "headless-" + host.ID + "-2", Valid: true},
			MaxRows: int32(100),
		})
		require.NoError(t, err)
		assert.Empty(t, logsAfter2, "logs for instance 2 should be deleted")
	})

	t.Run("成功: 最小権限 caller (host:write) で削除", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-delhost"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-delhost-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-delhost-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		setup.mockHostConnector.EXPECT().Remove(gomock.Any(), gomock.Any()).Return(nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: host.ID,
		}, "U-mp-delhost", groupID, []string{entity.PermKey_HostWrite})

		_, err := client.DeleteHeadlessHost(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が host 存在を先に確認するため、
	// 存在しないホストは NotFound を返す.
	t.Run("失敗: 存在しないホストは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: "nonexist",
		})

		_, err := client.DeleteHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_AllowHostAccess(t *testing.T) {
	t.Run("成功: ホストアクセスを許可", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			AllowHostAccess(gomock.Any(), gomock.Any()).
			Return(&headlessv1.AllowHostAccessResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.AllowHostAccessRequest{},
		})

		res, err := client.AllowHostAccess(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-allow"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-allow-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-allow-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().AllowHostAccess(gomock.Any(), gomock.Any()).Return(&headlessv1.AllowHostAccessResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.AllowHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.AllowHostAccessRequest{},
		}, "U-mp-allow", groupID, []string{entity.PermKey_HostWrite})

		_, err := client.AllowHostAccess(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId: host.ID,

			Request: &headlessv1.AllowHostAccessRequest{},
		})

		_, err := client.AllowHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: RPC呼び出しに失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			AllowHostAccess(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AllowHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.AllowHostAccessRequest{},
		})

		_, err := client.AllowHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_DenyHostAccess(t *testing.T) {
	t.Run("成功: ホストアクセスを拒否", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			DenyHostAccess(gomock.Any(), gomock.Any()).
			Return(&headlessv1.DenyHostAccessResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DenyHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.DenyHostAccessRequest{},
		})

		res, err := client.DenyHostAccess(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: 最小権限 caller (host:write) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-deny"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-deny-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-deny-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().DenyHostAccess(gomock.Any(), gomock.Any()).Return(&headlessv1.DenyHostAccessResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.DenyHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.DenyHostAccessRequest{},
		}, "U-mp-deny", groupID, []string{entity.PermKey_HostWrite})

		_, err := client.DenyHostAccess(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DenyHostAccessRequest{
			HostId:  host.ID,
			Request: &headlessv1.DenyHostAccessRequest{},
		})

		_, err := client.DenyHostAccess(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}
