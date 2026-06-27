package rpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
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
	t.Run("成功: ホストを起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")

		setup.mockHostConnector.EXPECT().
			Start(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, params hostconnector.HostStartParams) (hostconnector.HostConnectString, error) {
				assert.Equal(t, int32(1), params.InstanceId, "Initial start should have InstanceId=1")

				return hostconnector.HostConnectString("test-container"), nil
			})

		imageTag := "latest"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartHeadlessHostRequest{
			HeadlessAccountId: "U-test",
			Name:              "TestHost",
			ImageTag:          &imageTag,
		})

		res, err := client.StartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotEmpty(t, res.Msg.GetHostId())

		// Verify host was created in database
		host, err := setup.queries.GetHost(t.Context(), res.Msg.GetHostId())
		require.NoError(t, err)
		assert.Equal(t, "TestHost", host.Name)
		assert.Equal(t, "U-test", host.AccountID)
		assert.Equal(t, int32(1), host.InstanceCount, "Instance count should be 1 after first start")

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{
				ResoniteVersion: "1.0.0",
				AppVersion:      "1.0.0",
			}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{
				UserId:      "U-test",
				DisplayName: "Test Account",
			}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{
				Fps: 60.0,
			}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil).
			AnyTimes()

		getReq := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: res.Msg.GetHostId(),
		})

		getRes, err := client.GetHeadlessHost(t.Context(), getReq)
		require.NoError(t, err)
		assert.Equal(t, int32(1), getRes.Msg.GetHost().GetInstanceId(), "RPC response should include instance_id=1")
	})

	t.Run("失敗: 存在しないアカウント", func(t *testing.T) {
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
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_RestartHeadlessHost(t *testing.T) {
	t.Run("成功: ホストを再起動", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - ListContainerTags (called by resolveTagToUse)
		setup.mockHostConnector.EXPECT().
			ListContainerTags(gomock.Any(), gomock.Any()).
			Return(port.ContainerImageList{
				{Tag: "latest", IsPreRelease: false},
			}, nil).
			Times(1)

		// Mock HostConnector - GetRpcClient (may be called multiple times)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC client to return about info
		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{
				ResoniteVersion: "1.0.0",
				AppVersion:      "1.0.0",
			}, nil).
			AnyTimes()

		// Mock RPC client to return account info
		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{
				UserId:      "U-test",
				DisplayName: "Test Account",
			}, nil).
			AnyTimes()

		// Mock RPC client to return status
		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{
				Fps: 60.0,
			}, nil).
			AnyTimes()

		// Mock RPC client to return startup config
		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil).
			AnyTimes()

		// Mock RPC client to return sessions (for SearchSessions)
		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil).
			AnyTimes()

		// Mock HostConnector - Stop
		setup.mockHostConnector.EXPECT().
			Stop(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)

		// 旧コンテナの削除 (issue #21 対応)
		setup.mockHostConnector.EXPECT().
			Remove(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)

		setup.mockHostConnector.EXPECT().
			Start(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, params hostconnector.HostStartParams) (hostconnector.HostConnectString, error) {
				assert.Equal(t, int32(2), params.InstanceId, "Restart should have InstanceId=2")

				return hostconnector.HostConnectString("test-container-restarted"), nil
			}).
			Times(1)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.RestartHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.RestartHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		updatedHost, err := setup.queries.GetHost(t.Context(), host.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(2), updatedHost.InstanceCount, "Instance count should be 2 after restart")
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
	t.Run("成功: ホストをシャットダウン", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector - Stop calls GetRpcClient to fetch startup config
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil)

		setup.mockHostConnector.EXPECT().
			Stop(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
			HostId: host.ID,
		})

		res, err := client.ShutdownHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
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
	t.Run("成功: ホスト一覧を取得 (ページング検証)", func(t *testing.T) {
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

	t.Run("成功: 存在しないホストを削除（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteHeadlessHostRequest{
			HostId: "nonexist",
		})

		res, err := client.DeleteHeadlessHost(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
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
