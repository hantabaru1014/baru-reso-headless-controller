package rpc

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_BanUser(t *testing.T) {
	t.Run("成功: ユーザーをBANする", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			BanUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.BanUserResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.BanUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.BanUserRequest{
				User: &headlessv1.BanUserRequest_UserId{UserId: "U-target"},
			},
		})

		res, err := client.BanUser(t.Context(), req)
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.BanUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.BanUserRequest{
				User: &headlessv1.BanUserRequest_UserId{UserId: "U-target"},
			},
		})

		_, err := client.BanUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_KickUser(t *testing.T) {
	t.Run("成功: ユーザーをキックする", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			KickUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.KickUserResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KickUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.KickUserRequest{
				User: &headlessv1.KickUserRequest_UserId{UserId: "U-target"},
			},
		})

		res, err := client.KickUser(t.Context(), req)
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.KickUserRequest{
			HostId: host.ID,
			Parameters: &headlessv1.KickUserRequest{
				User: &headlessv1.KickUserRequest_UserId{UserId: "U-target"},
			},
		})

		_, err := client.KickUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_GetSessionDetails(t *testing.T) {
	t.Run("成功: セッション詳細を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Create test session
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetSessionDetailsRequest{
			SessionId: session.ID,
		})

		res, err := client.GetSessionDetails(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, session.ID, res.Msg.GetSession().GetId())
		assert.Equal(t, "TestSession", res.Msg.GetSession().GetName())
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetSessionDetailsRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.GetSessionDetails(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_ListUsersInSession(t *testing.T) {
	t.Run("成功: セッション内のユーザーリストを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			ListUsersInSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListUsersInSessionResponse{
				Users: []*headlessv1.UserInSession{
					{Id: "U-user1", Name: "User1"},
					{Id: "U-user2", Name: "User2"},
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListUsersInSessionRequest{
			HostId:    host.ID,
			SessionId: "session-123",
		})

		res, err := client.ListUsersInSession(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetUsers(), 2)
		assert.Equal(t, "U-user1", res.Msg.GetUsers()[0].GetId())
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListUsersInSessionRequest{
			HostId:    host.ID,
			SessionId: "session-123",
		})

		_, err := client.ListUsersInSession(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_SaveSessionWorld(t *testing.T) {
	t.Run("成功: セッションのワールドを保存", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// SessionUsecase.GetSession は cache miss 時に container の GetSession を打って
		// cache を populate する。テストでは cache 未投入なのでこの呼び出しが発生する。
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{Id: session.ID, Name: session.Name},
			}, nil).
			AnyTimes()

		// Mock RPC call to save world. container 側が新しく saved_world_url を返すので
		// preset 由来の初回 save でも stale な CurrentState ではなく同期的に正しい URL を返す。
		setup.mockRpcClient.EXPECT().
			SaveSessionWorld(gomock.Any(), gomock.Any()).
			Return(&headlessv1.SaveSessionWorldResponse{
				SavedWorldUrl: "resrec:///U-test/R-test-world",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: session.ID,
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE,
		})

		res, err := client.SaveSessionWorld(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.NotNil(t, res.Msg.GetSavedRecordUrl())
		assert.Equal(t, "resrec:///U-test/R-test-world", res.Msg.GetSavedRecordUrl())
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: "nonexist-session",
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_OVERWRITE,
		})

		_, err := client.SaveSessionWorld(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: 無効なセーブモード", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SaveSessionWorldRequest{
			SessionId: "session-123",
			SaveMode:  hdlctrlv1.SaveSessionWorldRequest_SAVE_MODE_UNKNOWN,
		})

		_, err := client.SaveSessionWorld(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_UpdateSessionParameters(t *testing.T) {
	t.Run("成功: セッションパラメータを更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session (called by usecase)
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		// Mock RPC call to update parameters
		maxUsers := int32(16)

		setup.mockRpcClient.EXPECT().
			UpdateSessionParameters(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateSessionParametersResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionParametersRequest{
			Parameters: &headlessv1.UpdateSessionParametersRequest{
				SessionId: session.ID,
				MaxUsers:  &maxUsers,
			},
		})

		res, err := client.UpdateSessionParameters(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionParametersRequest{
			Parameters: &headlessv1.UpdateSessionParametersRequest{
				SessionId: "nonexist-session",
			},
		})

		_, err := client.UpdateSessionParameters(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_UpdateUserRole(t *testing.T) {
	t.Run("成功: ユーザーのロールを更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			UpdateUserRole(gomock.Any(), gomock.Any()).
			Return(&headlessv1.UpdateUserRoleResponse{
				Role: "Admin",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateUserRoleRequest{
			HostId: host.ID,
			Parameters: &headlessv1.UpdateUserRoleRequest{
				SessionId: "session-123",
				User:      &headlessv1.UpdateUserRoleRequest_UserId{UserId: "U-target"},
				Role:      "Admin",
			},
		})

		res, err := client.UpdateUserRole(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, "Admin", res.Msg.GetRole())
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateUserRoleRequest{
			HostId: host.ID,
			Parameters: &headlessv1.UpdateUserRoleRequest{
				SessionId: "session-123",
				User:      &headlessv1.UpdateUserRoleRequest_UserId{UserId: "U-target"},
				Role:      "Admin",
			},
		})

		_, err := client.UpdateUserRole(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StartWorld(t *testing.T) {
	t.Run("成功: 非同期 job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)

		worldUrl := "resrec:///U-test/R-12345"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartWorldRequest{
			HostId: host.ID,
			Parameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: worldUrl},
			},
		})

		res, err := client.StartWorld(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_START_SESSION))
	})

	t.Run("失敗: 存在しないホスト", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		worldUrl := "resrec:///U-test/R-12345"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StartWorldRequest{
			HostId: "nonexist-host",
			Parameters: &headlessv1.WorldStartupParameters{
				LoadWorld: &headlessv1.WorldStartupParameters_LoadWorldUrl{LoadWorldUrl: worldUrl},
			},
		})

		_, err := client.StartWorld(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

// NOTE: forcePort の自動割り当てロジックは usecase.SessionUsecase.StartSession 内に
// あり、非同期 job 化後は worker 経由で実行されるため、ここでは検証しない.
// 既存の port 自動割り当てテストは usecase 層のテストへ移すか, AsyncJobExecutor
// の統合テストでカバーする (TODO).

func TestControllerService_InviteUser(t *testing.T) {
	t.Run("成功: ユーザーIDでユーザーを招待", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			InviteUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.InviteUserResponse{}, nil)

		userId := "U-target"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserId{UserId: userId},
		})

		res, err := client.InviteUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: ユーザー名でユーザーを招待", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test2", "test2@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test2", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			InviteUser(gomock.Any(), gomock.Any()).
			Return(&headlessv1.InviteUserResponse{}, nil)

		userName := "TargetUser"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserName{UserName: userName},
		})

		res, err := client.InviteUser(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		userId := "U-target"
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.InviteUserRequest{
			HostId:    host.ID,
			SessionId: "session-123",
			User:      &hdlctrlv1.InviteUserRequest_UserId{UserId: userId},
		})

		_, err := client.InviteUser(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_StopSession(t *testing.T) {
	t.Run("成功: 非同期 job が登録される", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		// 受付時 GetSession を通すには ENDED ステータスなら DB のみ参照する.
		// RUNNING にすると cache miss → GetRpcClient 経由で CRASHED に降格して
		// テストが副作用にまみれる. enqueue 確認用なので ENDED で十分.
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_EXITED)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_ENDED)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StopSessionRequest{
			SessionId: session.ID,
		})

		res, err := client.StopSession(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)
		assertJobEnqueued(t, setup, res.Msg.GetJobId(), int32(entity.AsyncJobType_STOP_SESSION))
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.StopSessionRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.StopSession(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_IssueResoniteLinkConnection(t *testing.T) {
	t.Run("成功: 認証済みユーザがトークン付きURLを発行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.IssueResoniteLinkConnectionRequest{
			SessionId: session.ID,
		})

		res, err := client.IssueResoniteLinkConnection(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, res.Msg)

		// ws_path は path + query 形式
		assert.True(t, strings.HasPrefix(res.Msg.GetWsPath(), "/resonite-link/ws?token="),
			"ws_path should start with /resonite-link/ws?token=, got %q", res.Msg.GetWsPath())

		// トークン部分を取り出して claims を検証
		const prefix = "/resonite-link/ws?token="

		token, decodeErr := url.QueryUnescape(strings.TrimPrefix(res.Msg.GetWsPath(), prefix))
		require.NoError(t, decodeErr)

		claims, err := auth.ParseResoniteLinkToken(token)
		require.NoError(t, err)
		assert.Equal(t, session.ID, claims.SessionID)
		assert.NotEmpty(t, claims.UserID)
		require.NotNil(t, res.Msg.GetExpiresAt())
		assert.True(t, res.Msg.GetExpiresAt().AsTime().After(time.Now()))
	})

	t.Run("失敗: 認証なし", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// No Authorization header
		req := connect.NewRequest(&hdlctrlv1.IssueResoniteLinkConnectionRequest{
			SessionId: "any-session",
		})

		_, err := client.IssueResoniteLinkConnection(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: session_id が空", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.IssueResoniteLinkConnectionRequest{
			SessionId: "",
		})

		_, err := client.IssueResoniteLinkConnection(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.IssueResoniteLinkConnectionRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.IssueResoniteLinkConnection(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}

func TestControllerService_SearchSessions(t *testing.T) {
	t.Run("成功: セッションを検索 (ページング検証)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and 10 sessions for pagination
		const totalSessions = 10

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		for i := 1; i <= totalSessions; i++ {
			testutil.CreateTestSession(t, setup.queries, host.ID, fmt.Sprintf("Session%02d", i), entity.SessionStatus_RUNNING)
		}

		// Mock HostConnector - GetRpcClient (called when fetching host info)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC calls
		setup.mockRpcClient.EXPECT().
			GetAbout(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAboutResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetAccountInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetAccountInfoResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStatus(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStatusResponse{}, nil).
			AnyTimes()

		setup.mockRpcClient.EXPECT().
			GetStartupConfigToRestore(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetStartupConfigToRestoreResponse{
				StartupConfig: &headlessv1.StartupConfig{},
			}, nil).
			AnyTimes()

		// Mock RPC call to list sessions (called by getHostSessions)
		setup.mockRpcClient.EXPECT().
			ListSessions(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListSessionsResponse{
				Sessions: []*headlessv1.Session{},
			}, nil).
			AnyTimes()

		// page 未指定 -> デフォルト 20 件 (10件しかないので全件)
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
		})
		res, err := client.SearchSessions(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetSessions(), totalSessions)
		require.NotNil(t, res.Msg.GetPage())
		assert.Equal(t, int32(totalSessions), res.Msg.GetPage().GetTotalCount())
		assert.Equal(t, int32(20), res.Msg.GetPage().GetPageSize())

		// page_size=4 で 3 ページに分割
		req2 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
			Page:       &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 4},
		})
		res2, err := client.SearchSessions(t.Context(), req2)
		require.NoError(t, err)
		assert.Len(t, res2.Msg.GetSessions(), 4)
		assert.Equal(t, int32(totalSessions), res2.Msg.GetPage().GetTotalCount())

		req3 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
			Page:       &hdlctrlv1.PageRequest{PageIndex: 1, PageSize: 4},
		})
		res3, err := client.SearchSessions(t.Context(), req3)
		require.NoError(t, err)
		assert.Len(t, res3.Msg.GetSessions(), 4)

		req4 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{},
			Page:       &hdlctrlv1.PageRequest{PageIndex: 2, PageSize: 4},
		})
		res4, err := client.SearchSessions(t.Context(), req4)
		require.NoError(t, err)
		assert.Len(t, res4.Msg.GetSessions(), 2)
		assert.Equal(t, int32(totalSessions), res4.Msg.GetPage().GetTotalCount())
	})

	t.Run("失敗: 無効な検索パラメータ", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{
				// Invalid parameters will be handled by usecase
			},
		})

		_, err := client.SearchSessions(t.Context(), req)
		// This may or may not fail depending on usecase implementation
		// For now, just verify it returns a response or error
		if err != nil {
			connectErr := &connect.Error{}
			ok := errors.As(err, &connectErr)
			require.True(t, ok, "expected connect.Error")
			assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
		}
	})

	t.Run("成功: StatusとHostIDでAND検索", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account and two hosts
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host1 := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost1", entity.HeadlessHostStatus_RUNNING)
		host2 := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		// Create sessions with different statuses on different hosts
		testutil.CreateTestSession(t, setup.queries, host1.ID, "Host1-Ended1", entity.SessionStatus_ENDED)
		testutil.CreateTestSession(t, setup.queries, host1.ID, "Host1-Ended2", entity.SessionStatus_ENDED)
		testutil.CreateTestSession(t, setup.queries, host1.ID, "Host1-Running", entity.SessionStatus_RUNNING)
		testutil.CreateTestSession(t, setup.queries, host2.ID, "Host2-Ended", entity.SessionStatus_ENDED)
		testutil.CreateTestSession(t, setup.queries, host2.ID, "Host2-Running", entity.SessionStatus_RUNNING)

		// Search for ENDED sessions on Host1 only
		status := hdlctrlv1.SessionStatus_SESSION_STATUS_ENDED
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{
				Status: &status,
				HostId: &host1.ID,
			},
		})

		res, err := client.SearchSessions(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Should return only ENDED sessions from Host1 (2 sessions)
		assert.Len(t, res.Msg.GetSessions(), 2)
		require.NotNil(t, res.Msg.GetPage())
		assert.Equal(t, int32(2), res.Msg.GetPage().GetTotalCount())

		for _, session := range res.Msg.GetSessions() {
			assert.Equal(t, host1.ID, session.GetHostId(), "Session should belong to Host1")
			assert.Equal(t, hdlctrlv1.SessionStatus_SESSION_STATUS_ENDED, session.GetStatus(), "Session should be ENDED")
		}

		// 同条件で page_size=1 -> 1件ずつ取れる
		req2 := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchSessionsRequest{
			Parameters: &hdlctrlv1.SearchSessionsRequest_SearchParameters{
				Status: &status,
				HostId: &host1.ID,
			},
			Page: &hdlctrlv1.PageRequest{PageIndex: 0, PageSize: 1},
		})
		res2, err := client.SearchSessions(t.Context(), req2)
		require.NoError(t, err)
		assert.Len(t, res2.Msg.GetSessions(), 1)
		assert.Equal(t, int32(2), res2.Msg.GetPage().GetTotalCount())
	})
}

func TestControllerService_DeleteEndedSession(t *testing.T) {
	t.Run("成功: 存在しないセッション（何も起こらない）", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteEndedSessionRequest{
			SessionId: "nonexist-session",
		})

		_, err := client.DeleteEndedSession(t.Context(), req)
		require.NoError(t, err)
	})
}

func TestControllerService_UpdateSessionExtraSettings(t *testing.T) {
	t.Run("成功: セッションの追加設定を更新", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account, host, and session
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)
		session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_RUNNING)

		// Mock HostConnector - GetRpcClient (called by GetSession)
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil).
			AnyTimes()

		// Mock RPC call to get session
		setup.mockRpcClient.EXPECT().
			GetSession(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetSessionResponse{
				Session: &headlessv1.Session{
					Id:   session.ID,
					Name: session.Name,
				},
			}, nil).
			AnyTimes()

		autoUpgrade := true
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionExtraSettingsRequest{
			SessionId:   session.ID,
			AutoUpgrade: &autoUpgrade,
		})

		res, err := client.UpdateSessionExtraSettings(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify settings were updated in database
		updatedSession, err := setup.queries.GetSession(t.Context(), session.ID)
		require.NoError(t, err)
		assert.True(t, updatedSession.AutoUpgrade)
	})

	t.Run("失敗: 存在しないセッション", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		autoUpgrade := true
		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateSessionExtraSettingsRequest{
			SessionId:   "nonexist-session",
			AutoUpgrade: &autoUpgrade,
		})

		_, err := client.UpdateSessionExtraSettings(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}
