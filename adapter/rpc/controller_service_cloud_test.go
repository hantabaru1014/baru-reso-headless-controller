package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_AcceptFriendRequests(t *testing.T) {
	t.Run("成功: フレンドリクエストを承認", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-friend", "friend@example.test", "password")

		// Create test host in database
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-friend", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to accept friend requests
		setup.mockRpcClient.EXPECT().
			AcceptFriendRequests(gomock.Any(), gomock.Any()).
			Return(&headlessv1.AcceptFriendRequestsResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-friend",
			TargetUserId:      "U-target",
		})

		res, err := client.AcceptFriendRequests(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)

		// Verify host was actually created
		_, err = setup.queries.GetHost(t.Context(), host.ID)
		require.NoError(t, err)
	})

	t.Run("成功: 最小権限 caller (account:use) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-accept"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-acc-accept", "x@example.test", "p", groupID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-acc-accept", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().AcceptFriendRequests(gomock.Any(), gomock.Any()).Return(&headlessv1.AcceptFriendRequestsResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-mp-acc-accept",
			TargetUserId:      "U-target",
		}, "U-mp-accept", groupID, []string{entity.PermKey_AccountUse})

		_, err := client.AcceptFriendRequests(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: 起動中のホストがない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-nohost", "nohost@example.test", "password")

		// No running hosts created, so ListRunningByAccount will return empty list

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-nohost",
			TargetUserId:      "U-target",
		})

		_, err := client.AcceptFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: RPCクライアントの取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)
		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-rpcfail", "rpcfail@example.test", "password")

		// Create test host
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-rpcfail", "TestHost2", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return error when getting RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.AcceptFriendRequestsRequest{
			HeadlessAccountId: "U-rpcfail",
			TargetUserId:      "U-target",
		})

		_, err := client.AcceptFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_GetFriendRequests(t *testing.T) {
	t.Run("成功: フレンドリクエストを取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-friend", "friend@example.test", "password")

		// Mock skyfrost to return friend requests
		setup.mockSkyfrost.EXPECT().
			GetContacts(gomock.Any(), "friend@example.test", "password").
			Return([]skyfrost.Contact{
				{
					Id:         "contact1",
					Username:   "User1",
					Status:     "Requested",
					IsAccepted: false,
				},
				{
					Id:         "contact2",
					Username:   "User2",
					Status:     "Requested",
					IsAccepted: false,
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-friend",
		})

		res, err := client.GetFriendRequests(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetRequestedContacts(), 2)
	})

	t.Run("成功: 最小権限 caller (account:use) で取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-friendreq"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-fr", "fr@example.test", "p", groupID)

		setup.mockSkyfrost.EXPECT().
			GetContacts(gomock.Any(), "fr@example.test", "p").
			Return([]skyfrost.Contact{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-mp-fr",
		}, "U-mp-friendreq", groupID, []string{entity.PermKey_AccountUse})

		_, err := client.GetFriendRequests(t.Context(), req)
		require.NoError(t, err)
	})

	// 権限システム導入後は permission interceptor が account の所属グループを
	// 確認するため、存在しないアカウントは NotFound を返す.
	t.Run("失敗: 存在しないアカウントは NotFound", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-nonexist",
		})

		_, err := client.GetFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: コンタクト取得に失敗", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test account
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-contactfail", "user@example.test", "password")

		// Mock skyfrost to return contacts error
		setup.mockSkyfrost.EXPECT().
			GetContacts(gomock.Any(), "user@example.test", "password").
			Return(nil, connect.NewError(connect.CodeInternal, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetFriendRequestsRequest{
			HeadlessAccountId: "U-contactfail",
		})

		_, err := client.GetFriendRequests(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)

		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_SearchUserInfo(t *testing.T) {
	t.Run("成功: ユーザー情報を検索", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			SearchUserInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.SearchUserInfoResponse{
				Users: []*headlessv1.UserInfo{
					{
						Id:   "U-found",
						Name: "FoundUser",
					},
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchUserInfoRequest{
			HostId: host.ID,
			Parameters: &headlessv1.SearchUserInfoRequest{
				User: &headlessv1.SearchUserInfoRequest_UserName{UserName: "test"},
			},
		})

		res, err := client.SearchUserInfo(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Len(t, res.Msg.GetUsers(), 1)
		assert.Equal(t, "U-found", res.Msg.GetUsers()[0].GetId())
	})

	t.Run("成功: 最小権限 caller (host:use) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-suserinfo"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-su-acc", "x@example.test", "p", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-su-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().SearchUserInfo(gomock.Any(), gomock.Any()).Return(&headlessv1.SearchUserInfoResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.SearchUserInfoRequest{
			HostId: host.ID,
			Parameters: &headlessv1.SearchUserInfoRequest{
				User: &headlessv1.SearchUserInfoRequest_UserName{UserName: "test"},
			},
		}, "U-mp-suserinfo", groupID, []string{entity.PermKey_HostUse})

		_, err := client.SearchUserInfo(t.Context(), req)
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SearchUserInfoRequest{
			HostId: host.ID,
			Parameters: &headlessv1.SearchUserInfoRequest{
				User: &headlessv1.SearchUserInfoRequest_UserName{UserName: "test"},
			},
		})

		_, err := client.SearchUserInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})
}

func TestControllerService_FetchWorldInfo(t *testing.T) {
	t.Run("成功: ワールド情報を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test", "test@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test", "TestHost", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			FetchWorldInfo(gomock.Any(), gomock.Any()).
			Return(&headlessv1.FetchWorldInfoResponse{
				Name: "TestWorld",
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-12345",
		})

		res, err := client.FetchWorldInfo(t.Context(), req)

		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
		assert.Equal(t, "TestWorld", res.Msg.GetName())
	})

	t.Run("成功: 最小権限 caller (host:use) で実行", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-fworld"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-fw-acc", "x@example.test", "p", groupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-fw-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().FetchWorldInfo(gomock.Any(), gomock.Any()).Return(&headlessv1.FetchWorldInfoResponse{Name: "MP"}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-mp/R-12345",
		}, "U-mp-fworld", groupID, []string{entity.PermKey_HostUse})

		_, err := client.FetchWorldInfo(t.Context(), req)
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

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-12345",
		})

		_, err := client.FetchWorldInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInternal, connectErr.Code())
	})

	t.Run("失敗: ワールドが見つからない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-test3", "test3@example.test", "password")
		host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-test3", "TestHost3", entity.HeadlessHostStatus_RUNNING)

		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		setup.mockRpcClient.EXPECT().
			FetchWorldInfo(gomock.Any(), gomock.Any()).
			Return(nil, connect.NewError(connect.CodeNotFound, nil))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.FetchWorldInfoRequest{
			HostId: host.ID,
			Url:    "resrec:///U-test/R-nonexist",
		})

		_, err := client.FetchWorldInfo(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})
}
