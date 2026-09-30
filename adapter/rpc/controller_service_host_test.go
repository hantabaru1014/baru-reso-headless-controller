package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_ListHeadlessHostImageTags(t *testing.T) {
	t.Run("成功: DB の built 済み resonite_versions を返す", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		ctx := t.Context()
		// build 済み 1 件 (headless)
		gv1 := "2024.1.1"
		require.NoError(t, seedBuiltResoVersion(ctx, setup.queries, "MANIFEST-1", "headless", gv1, "2024.1.1-v1.0.0", "v1.0.0"))
		// build 済み 1 件 (prerelease)
		gv2 := "2024.1.2"
		require.NoError(t, seedBuiltResoVersion(ctx, setup.queries, "MANIFEST-2", "prerelease", gv2, "prerelease-2024.1.2-v1.1.0", "v1.1.0"))
		// 未 built (返却対象外)
		require.NoError(t, seedNotBuiltResoVersion(ctx, setup.queries, "MANIFEST-3", "headless", "2024.1.3"))
		// DB 上 built だがローカル image が prune 済み (返却対象外)
		require.NoError(t, seedBuiltResoVersion(ctx, setup.queries, "MANIFEST-4", "headless", "2024.1.4", "2024.1.4-v1.0.0", "v1.0.0"))

		// ローカルに実在する image は MANIFEST-1 / MANIFEST-2 の 2 つのみ.
		setup.mockHostConnector.EXPECT().ListLocalImageTags(gomock.Any()).
			Return([]string{"2024.1.1-v1.0.0", "prerelease-2024.1.2-v1.1.0"}, nil).AnyTimes()

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListHeadlessHostImageTagsRequest{})

		res, err := client.ListHeadlessHostImageTags(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetTags(), 2)

		gotTags := map[string]*hdlctrlv1.ListHeadlessHostImageTagsResponse_ContainerImage{}
		for _, t := range res.Msg.GetTags() {
			gotTags[t.GetTag()] = t
		}

		require.Contains(t, gotTags, "2024.1.1-v1.0.0")
		assert.Equal(t, "2024.1.1", gotTags["2024.1.1-v1.0.0"].GetResoniteVersion())
		assert.False(t, gotTags["2024.1.1-v1.0.0"].GetIsPrerelease())
		assert.Equal(t, "v1.0.0", gotTags["2024.1.1-v1.0.0"].GetAppVersion())

		require.Contains(t, gotTags, "prerelease-2024.1.2-v1.1.0")
		assert.True(t, gotTags["prerelease-2024.1.2-v1.1.0"].GetIsPrerelease())
		assert.Equal(t, "v1.1.0", gotTags["prerelease-2024.1.2-v1.1.0"].GetAppVersion())

		// image が消えたタグは起動候補から除外される.
		assert.NotContains(t, gotTags, "2024.1.4-v1.0.0")

		// ListResoniteVersions では image が消えた built 行は not_built として返る
		// (フロントが再ビルド候補として扱えるように).
		lvRes, err := client.ListResoniteVersions(ctx,
			testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListResoniteVersionsRequest{}))
		require.NoError(t, err)

		statusByManifest := map[string]hdlctrlv1.ResoniteVersionBuildStatus{}
		for _, v := range lvRes.Msg.GetVersions() {
			statusByManifest[v.GetManifestId()] = v.GetBuildStatus()
		}

		assert.Equal(t, hdlctrlv1.ResoniteVersionBuildStatus_RESONITE_VERSION_BUILD_STATUS_BUILT, statusByManifest["MANIFEST-1"])
		assert.Equal(t, hdlctrlv1.ResoniteVersionBuildStatus_RESONITE_VERSION_BUILD_STATUS_NOT_BUILT, statusByManifest["MANIFEST-4"])
	})
}

func seedBuiltResoVersion(ctx context.Context, q *db.Queries, manifestID, branch, gameVersion, imageTag, appVersion string) error {
	if _, err := q.UpsertResoniteVersion(ctx, db.UpsertResoniteVersionParams{
		ManifestID:  manifestID,
		Branch:      branch,
		GameVersion: pgtype.Text{String: gameVersion, Valid: true},
		ReleasedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return err
	}

	if _, err := q.SetResoniteVersionBuilt(ctx, db.SetResoniteVersionBuiltParams{
		ManifestID:          manifestID,
		Branch:              branch,
		ImageTag:            imageTag,
		BuiltWithAppVersion: appVersion,
	}); err != nil {
		return err
	}

	return nil
}

func seedNotBuiltResoVersion(ctx context.Context, q *db.Queries, manifestID, branch, gameVersion string) error {
	_, err := q.UpsertResoniteVersion(ctx, db.UpsertResoniteVersionParams{
		ManifestID:  manifestID,
		Branch:      branch,
		GameVersion: pgtype.Text{String: gameVersion, Valid: true},
		ReleasedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})

	return err
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

func TestControllerService_BuildResoniteImage(t *testing.T) {
	t.Run("成功: system-admin で BUILD_IMAGE job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
		})

		res, err := client.BuildResoniteImage(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_BUILD_IMAGE))
	})

	t.Run("成功: いずれかのグループの host:write のみで実行できる", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
		}, "U-mp-build", "g-mp-build", []string{entity.PermKey_HostWrite})

		res, err := client.BuildResoniteImage(t.Context(), req)
		require.NoError(t, err)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_BUILD_IMAGE))
	})

	t.Run("失敗: host:write をどのグループにも持たないと PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
		}, "U-mp-build-nowrite", "g-mp-build-nowrite", []string{entity.PermKey_HostRead})

		_, err := client.BuildResoniteImage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_HostWrite)
	})

	t.Run("成功: then_start_host 付きは host:write + account:use で実行できる", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-build-chain"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
			FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenStartHost{
				ThenStartHost: &hdlctrlv1.StartHeadlessHostRequest{
					HeadlessAccountId: "U-mp-acc",
					Name:              "TestHost",
				},
			},
		}, "U-mp-build-chain", groupID, []string{
			entity.PermKey_HostWrite,
			entity.PermKey_AccountUse,
		})

		res, err := client.BuildResoniteImage(t.Context(), req)
		require.NoError(t, err)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_BUILD_IMAGE))
	})

	t.Run("失敗: then_start_host 付きで account:use 不足なら PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-build-chain-noaccuse"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
			FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenStartHost{
				ThenStartHost: &hdlctrlv1.StartHeadlessHostRequest{
					HeadlessAccountId: "U-mp-acc",
					Name:              "TestHost",
				},
			},
		}, "U-mp-build-chain-noaccuse", groupID, []string{entity.PermKey_HostWrite})

		_, err := client.BuildResoniteImage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_AccountUse)
	})

	t.Run("成功: then_restart_host 付きは対象ホストの host:write で実行できる", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-build-restart-chain"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc", "mp@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-acc", "TestHost", entity.HeadlessHostStatus_EXITED, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
			FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenRestartHost{
				ThenRestartHost: &hdlctrlv1.RestartHeadlessHostRequest{HostId: host.ID},
			},
		}, "U-mp-build-restart-chain", groupID, []string{entity.PermKey_HostWrite})

		res, err := client.BuildResoniteImage(t.Context(), req)
		require.NoError(t, err)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_BUILD_IMAGE))
	})

	// build 単体を実行できる権限では、他グループのホストを再起動させる chain までは通せない.
	t.Run("失敗: then_restart_host 付きで対象ホストの host:write が無いと PermissionDenied", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const callerGroupID = "g-mp-build-restart-caller"

		const hostGroupID = "g-mp-build-restart-other"

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-other-acc", "other@example.test", "password", hostGroupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-other-acc", "OtherHost", entity.HeadlessHostStatus_EXITED, hostGroupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.BuildResoniteImageRequest{
			ManifestId: "MANIFEST-1",
			Branch:     "headless",
			FollowUp: &hdlctrlv1.BuildResoniteImageRequest_ThenRestartHost{
				ThenRestartHost: &hdlctrlv1.RestartHeadlessHostRequest{HostId: host.ID},
			},
		}, "U-mp-build-restart-caller", callerGroupID, []string{entity.PermKey_HostWrite})

		_, err := client.BuildResoniteImage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_HostWrite)
	})
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

// logBodies はログ本文だけを取り出す. どのログが返ったかを順序込みで検証するためのもの.
func logBodies(logs []*hdlctrlv1.GetHeadlessHostLogsResponse_Log) []string {
	bodies := make([]string, 0, len(logs))
	for _, log := range logs {
		bodies = append(bodies, log.GetBody())
	}

	return bodies
}

// logIDsByBody は指定インスタンスの全ログを取得し、本文 → ID の対応を返す.
func logIDsByBody(t *testing.T, client hdlctrlv1connect.ControllerServiceClient, hostID string, instanceID int32) map[string]int64 {
	t.Helper()

	res, err := client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
		HostId:     hostID,
		InstanceId: instanceID,
	}))
	require.NoError(t, err)

	ids := make(map[string]int64, len(res.Msg.GetLogs()))
	for _, log := range res.Msg.GetLogs() {
		ids[log.GetBody()] = log.GetId()
	}

	return ids
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
		// 最新のログを含む末尾 3 件が時系列順で返る (余分に取得した 1 件は古い側を捨てる)
		assert.Equal(t, []string{"Log line 3", "Log line 4", "Log line 5"}, logBodies(res.Msg.GetLogs()))
		assert.True(t, res.Msg.GetHasMoreBefore(), "should have more logs before")
		assert.False(t, res.Msg.GetHasMoreAfter(), "should not have more logs after (initial fetch)")

		// 最新ログを before_id にすると、その直前の 3 件が返る (カーソルに隣接するログを捨てない)
		latestID := res.Msg.GetLogs()[2].GetId()
		beforeLatestRes, err := client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      3,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_BeforeId{BeforeId: latestID},
		}))
		require.NoError(t, err)
		assert.Equal(t, []string{"Log line 2", "Log line 3", "Log line 4"}, logBodies(beforeLatestRes.Msg.GetLogs()))
		assert.True(t, beforeLatestRes.Msg.GetHasMoreBefore(), "Log line 1 remains")

		// 1 ページ目の先頭を before_id にして続きを取得すると、抜けなく残りが返る
		oldestID := res.Msg.GetLogs()[0].GetId()
		nextPageRes, err := client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      3,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_BeforeId{BeforeId: oldestID},
		}))
		require.NoError(t, err)
		assert.Equal(t, []string{"Log line 1", "Log line 2"}, logBodies(nextPageRes.Msg.GetLogs()))
		assert.False(t, nextPageRes.Msg.GetHasMoreBefore(), "no more older logs")
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
		testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(3*time.Minute), "stdout", "Newer log")

		ids := logIDsByBody(t, client, host.ID, host.InstanceCount)
		require.Len(t, ids, 4)

		// Use afterId cursor to get logs after "Middle log"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AfterId{AfterId: ids["Middle log"]},
		})

		res, err := client.GetHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		// カーソルより新しいログが時系列順で返る
		assert.Equal(t, []string{"New log", "Newer log"}, logBodies(res.Msg.GetLogs()))
		assert.False(t, res.Msg.GetHasMoreAfter(), "no more newer logs")
		assert.False(t, res.Msg.GetHasMoreBefore(), "should not have has_more_before with after cursor")

		// after_id = 0 は「最古のログから」であり、カーソルなし (最新から) にはならない
		res, err = client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      2,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AfterId{AfterId: 0},
		}))
		require.NoError(t, err)
		assert.Equal(t, []string{"Old log", "Middle log"}, logBodies(res.Msg.GetLogs()))
		assert.True(t, res.Msg.GetHasMoreAfter())
		assert.False(t, res.Msg.GetHasMoreBefore())
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
		// カーソル直後の 2 件が時系列順で返る (最新側の 2 件ではない)
		assert.Equal(t, []string{"Log 2", "Log 3"}, logBodies(res.Msg.GetLogs()))
		assert.True(t, res.Msg.GetHasMoreAfter(), "should have more logs after")
		assert.False(t, res.Msg.GetHasMoreBefore(), "should not have has_more_before with after cursor")

		// 返ってきた末尾を after_id にして続きを取得すると、抜けなく残りが返る
		nextPageRes, err := client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Limit:      2,
			Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AfterId{AfterId: res.Msg.GetLogs()[1].GetId()},
		}))
		require.NoError(t, err)
		assert.Equal(t, []string{"Log 4", "Log 5"}, logBodies(nextPageRes.Msg.GetLogs()))
		assert.False(t, nextPageRes.Msg.GetHasMoreAfter(), "no more newer logs")
	})

	t.Run("成功: aroundIdカーソルで指定ログの前後を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test8", "test8@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test8", "TestHost8", entity.HeadlessHostStatus_EXITED)

		// Insert 10 logs. 間に別インスタンスのログを挟み、ID が連番でなくても正しく前後を取れることを確認する
		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		for i := range 10 {
			testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Duration(i)*time.Minute), "stdout", fmt.Sprintf("Log %d", i+1))
			testutil.InsertTestContainerLog(t, setup.queries, host.ID, 0, baseTime.Add(time.Duration(i)*time.Minute), "stdout", fmt.Sprintf("Other instance log %d", i+1))
		}

		ids := logIDsByBody(t, client, host.ID, host.InstanceCount)
		require.Len(t, ids, 10)

		getAround := func(target string, limit int32) *hdlctrlv1.GetHeadlessHostLogsResponse {
			res, err := client.GetHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Limit:      limit,
				Cursor:     &hdlctrlv1.GetHeadlessHostLogsRequest_AroundId{AroundId: ids[target]},
			}))
			require.NoError(t, err)

			return res.Msg
		}

		// 中央: 対象ログ以前を limit/2 件 (対象を含む)、対象より後を limit/2 件、時系列順で返す
		middle := getAround("Log 5", 4)
		assert.Equal(t, []string{"Log 4", "Log 5", "Log 6", "Log 7"}, logBodies(middle.GetLogs()))
		assert.True(t, middle.GetHasMoreBefore(), "Log 1-3 remain")
		assert.True(t, middle.GetHasMoreAfter(), "Log 8-10 remain")

		// limit が奇数の場合は対象ログを含む前半が 1 件多い
		odd := getAround("Log 5", 3)
		assert.Equal(t, []string{"Log 4", "Log 5", "Log 6"}, logBodies(odd.GetLogs()))
		assert.True(t, odd.GetHasMoreBefore())
		assert.True(t, odd.GetHasMoreAfter())

		// limit = 1 でも対象ログは返る
		single := getAround("Log 5", 1)
		assert.Equal(t, []string{"Log 5"}, logBodies(single.GetLogs()))
		assert.True(t, single.GetHasMoreBefore())
		assert.True(t, single.GetHasMoreAfter())

		// 先頭付近: 前半が足りなくても後半を増やさず、has_more_before は false
		first := getAround("Log 1", 4)
		assert.Equal(t, []string{"Log 1", "Log 2", "Log 3"}, logBodies(first.GetLogs()))
		assert.False(t, first.GetHasMoreBefore(), "no older logs than Log 1")
		assert.True(t, first.GetHasMoreAfter(), "Log 4-10 remain")

		// 前半がちょうど収まる場合も has_more_before は false
		second := getAround("Log 2", 4)
		assert.Equal(t, []string{"Log 1", "Log 2", "Log 3", "Log 4"}, logBodies(second.GetLogs()))
		assert.False(t, second.GetHasMoreBefore(), "Log 1 is the oldest")
		assert.True(t, second.GetHasMoreAfter(), "Log 5-10 remain")

		// 末尾付近: 後半が無くても対象ログは返り、has_more_after は false
		last := getAround("Log 10", 4)
		assert.Equal(t, []string{"Log 9", "Log 10"}, logBodies(last.GetLogs()))
		assert.True(t, last.GetHasMoreBefore(), "Log 1-8 remain")
		assert.False(t, last.GetHasMoreAfter(), "no newer logs than Log 10")

		// 後半がちょうど収まる場合も has_more_after は false
		secondLast := getAround("Log 8", 4)
		assert.Equal(t, []string{"Log 7", "Log 8", "Log 9", "Log 10"}, logBodies(secondLast.GetLogs()))
		assert.True(t, secondLast.GetHasMoreBefore(), "Log 1-6 remain")
		assert.False(t, secondLast.GetHasMoreAfter(), "Log 10 is the newest")

		// limit 未指定時はデフォルト件数 (100) で全件が収まる
		all := getAround("Log 5", 0)
		assert.Equal(t, []string{"Log 1", "Log 2", "Log 3", "Log 4", "Log 5", "Log 6", "Log 7", "Log 8", "Log 9", "Log 10"}, logBodies(all.GetLogs()))
		assert.False(t, all.GetHasMoreBefore())
		assert.False(t, all.GetHasMoreAfter())
	})
}

func TestControllerService_SearchHeadlessHostLogs(t *testing.T) {
	const backslashLog = `Loading C:\data\world.json`

	// 検索対象のログ (古い順). これらの後に別インスタンス (instance 0) のログを最新として入れる.
	searchTestLogs := []string{
		"Server started",
		backslashLog,
		"User Alice joined",
		"progress 100% done",
		"user_name changed",
		"USER Bob joined",
		"Shutdown complete",
	}

	const otherInstanceLog = "user joined other instance"

	// seedLogs はホストと検索対象のログを作成し、ホストと本文 → ID の対応を返す.
	seedLogs := func(t *testing.T, setup *controllerServiceTestSetup, client hdlctrlv1connect.ControllerServiceClient, groupID string) (db.Host, map[string]int64) {
		t.Helper()

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-search-acc", "search@example.test", "password", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-search-acc", "SearchHost", entity.HeadlessHostStatus_EXITED, groupID)

		baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		for i, body := range searchTestLogs {
			// 実際のコンテナログと同様に末尾へ改行を付ける
			testutil.InsertTestContainerLog(t, setup.queries, host.ID, host.InstanceCount, baseTime.Add(time.Duration(i)*time.Second), "stdout", body+"\n")
		}

		testutil.InsertTestContainerLog(t, setup.queries, host.ID, 0, baseTime.Add(time.Hour), "stdout", otherInstanceLog)

		ids := logIDsByBody(t, client, host.ID, host.InstanceCount)
		require.Len(t, ids, len(searchTestLogs))

		return host, ids
	}

	// search は system-admin として検索し、一致したログの ID (見つからなければ nil) を返す.
	search := func(t *testing.T, client hdlctrlv1connect.ControllerServiceClient, msg *hdlctrlv1.SearchHeadlessHostLogsRequest) *int64 {
		t.Helper()

		res, err := client.SearchHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, msg))
		require.NoError(t, err)

		return res.Msg.LogId
	}

	t.Run("成功: カーソルなしは最新の一致を返す (大文字小文字を区別しない)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, ids := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		// "user" は User Alice / user_name / USER Bob に一致する. 最新の USER Bob を返す
		got := search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "user",
		})
		require.NotNil(t, got)
		assert.Equal(t, ids["USER Bob joined"], *got)

		// クエリ側が大文字でも一致する
		got = search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "ALICE JOIN",
		})
		require.NotNil(t, got)
		assert.Equal(t, ids["User Alice joined"], *got)
	})

	t.Run("成功: 最小権限 caller (host:read) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-searchlogs"

		host, ids := seedLogs(t, setup, client, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "started",
		}, "U-mp-searchlogs", groupID, []string{entity.PermKey_HostRead})

		res, err := client.SearchHeadlessHostLogs(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg.LogId)
		assert.Equal(t, ids["Server started"], res.Msg.GetLogId())
	})

	t.Run("成功: beforeIdカーソルより古い方向で最も近い一致を返す", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, ids := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		searchBefore := func(beforeBody string) *int64 {
			return search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Query:      "user",
				Cursor:     &hdlctrlv1.SearchHeadlessHostLogsRequest_BeforeId{BeforeId: ids[beforeBody]},
			})
		}

		// カーソルのログ自身 (USER Bob) は一致していても対象外で、その直前の一致を返す
		got := searchBefore("USER Bob joined")
		require.NotNil(t, got)
		assert.Equal(t, ids["user_name changed"], *got)

		// 一致しないログをカーソルにした場合も、最古の一致 (User Alice) ではなく最も近い一致を返す
		got = searchBefore("Shutdown complete")
		require.NotNil(t, got)
		assert.Equal(t, ids["USER Bob joined"], *got)

		got = searchBefore("user_name changed")
		require.NotNil(t, got)
		assert.Equal(t, ids["User Alice joined"], *got)

		// それより古い一致は無い
		assert.Nil(t, searchBefore("User Alice joined"))
	})

	t.Run("成功: afterIdカーソルより新しい方向で最も近い一致を返す", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, ids := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		searchAfter := func(afterBody string) *int64 {
			return search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Query:      "user",
				Cursor:     &hdlctrlv1.SearchHeadlessHostLogsRequest_AfterId{AfterId: ids[afterBody]},
			})
		}

		// カーソルのログ自身 (User Alice) は対象外で、最新の一致 (USER Bob) ではなく直後の一致を返す
		got := searchAfter("User Alice joined")
		require.NotNil(t, got)
		assert.Equal(t, ids["user_name changed"], *got)

		got = searchAfter("Server started")
		require.NotNil(t, got)
		assert.Equal(t, ids["User Alice joined"], *got)

		got = searchAfter("user_name changed")
		require.NotNil(t, got)
		assert.Equal(t, ids["USER Bob joined"], *got)

		// それより新しい一致は無い (より新しい別インスタンスのログには一致しない)
		assert.Nil(t, searchAfter("USER Bob joined"))

		// after_id = 0 は「最古のログから新しい方向へ」であり、カーソルなし (最新から古い方向へ) にはならない
		got = search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "user",
			Cursor:     &hdlctrlv1.SearchHeadlessHostLogsRequest_AfterId{AfterId: 0},
		})
		require.NotNil(t, got)
		assert.Equal(t, ids["User Alice joined"], *got)
	})

	t.Run("成功: % と _ とバックスラッシュはワイルドカードではなく文字として一致する", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, ids := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		// ワイルドカード扱いなら最新の "Shutdown complete" に一致してしまう
		for query, wantBody := range map[string]string{
			"%":       "progress 100% done",
			"100% d":  "progress 100% done",
			"_":       "user_name changed",
			"r_n":     "user_name changed",
			`\`:       backslashLog,
			`c:\DATA`: backslashLog,
		} {
			got := search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Query:      query,
			})
			require.NotNil(t, got, "query %q should match", query)
			assert.Equal(t, ids[wantBody], *got, "query %q", query)
		}

		// "_" / "%" が任意文字扱い、"\" がエスケープ扱いなら一致してしまう
		for _, query := range []string{"User_Alice", "User%joined", `\U`, `C:\\data`} {
			assert.Nil(t, search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Query:      query,
			}), "query %q should not match", query)
		}
	})

	t.Run("成功: 一致するログが無い場合は log_id 未設定", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, _ := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		assert.Nil(t, search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "no such message",
		}))

		// 最大長 (200 文字) のクエリは受け付ける (マルチバイト文字はバイト数ではなく文字数で数える)
		assert.Nil(t, search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      strings.Repeat("あ", 200),
		}))
	})

	t.Run("成功: 別インスタンスのログには一致しない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, ids := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		// instance 0 にしか無い文言は、対象インスタンスでは見つからない
		assert.Nil(t, search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: host.InstanceCount,
			Query:      "other instance",
		}))

		// instance 0 を指定すれば instance 0 のログだけが対象になる
		got := search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: 0,
			Query:      "user",
		})
		require.NotNil(t, got)
		assert.Equal(t, logIDsByBody(t, client, host.ID, 0)[otherInstanceLog], *got)
		assert.NotEqual(t, ids["USER Bob joined"], *got)

		assert.Nil(t, search(t, client, &hdlctrlv1.SearchHeadlessHostLogsRequest{
			HostId:     host.ID,
			InstanceId: 0,
			Query:      "Shutdown",
		}))
	})

	t.Run("失敗: 空または長すぎる検索文字列は InvalidArgument", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		host, _ := seedLogs(t, setup, client, entity.MigratedPrePermissionGroupID)

		for _, query := range []string{"", strings.Repeat("a", 201), strings.Repeat("あ", 201)} {
			_, err := client.SearchHeadlessHostLogs(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     host.ID,
				InstanceId: host.InstanceCount,
				Query:      query,
			}))
			require.Error(t, err)

			connectErr := &connect.Error{}
			require.ErrorAs(t, err, &connectErr)
			assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code(), "query of %d bytes", len(query))
		}
	})

	t.Run("失敗: host:read の無いグループのホストは NotFound (存在しないホストと区別できない)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const (
			callerUserID  = "U-mp-searchlogs-stranger"
			callerGroupID = "g-mp-searchlogs-caller"
			hostGroupID   = "g-mp-searchlogs-other"
		)

		host, _ := seedLogs(t, setup, client, hostGroupID)

		// caller は自分のグループの host:read しか持たない
		testutil.SetupUserWithExactPermissions(t, setup.queries, callerUserID, callerGroupID, []string{entity.PermKey_HostRead})

		searchErr := func(hostID string) *connect.Error {
			req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.SearchHeadlessHostLogsRequest{
				HostId:     hostID,
				InstanceId: host.InstanceCount,
				Query:      "user",
			}, callerUserID, "U-resonite-"+callerUserID, "")

			_, err := client.SearchHeadlessHostLogs(t.Context(), req)
			require.Error(t, err)

			connectErr := &connect.Error{}
			require.ErrorAs(t, err, &connectErr)

			return connectErr
		}

		otherGroupErr := searchErr(host.ID)
		missingErr := searchErr("non-existent-host")

		assert.Equal(t, connect.CodeNotFound, otherGroupErr.Code())
		assert.Equal(t, missingErr.Code(), otherGroupErr.Code())
		assert.Equal(t, missingErr.Message(), otherGroupErr.Message())
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
		logsBefore, err := setup.queries.GetContainerLogsAfter(t.Context(), db.GetContainerLogsAfterParams{
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
		logsAfter1, err := setup.queries.GetContainerLogsAfter(t.Context(), db.GetContainerLogsAfterParams{
			Tag:     pgtype.Text{String: "headless-" + host.ID + "-1", Valid: true},
			MaxRows: int32(100),
		})
		require.NoError(t, err)
		assert.Empty(t, logsAfter1, "logs for instance 1 should be deleted")

		logsAfter2, err := setup.queries.GetContainerLogsAfter(t.Context(), db.GetContainerLogsAfterParams{
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
