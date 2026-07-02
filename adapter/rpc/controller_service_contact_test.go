package rpc

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestControllerService_ListContacts(t *testing.T) {
	t.Run("成功: コンタクト一覧を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-contact1", "contact@example.test", "password")
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-contact1", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to list contacts
		setup.mockRpcClient.EXPECT().
			ListContacts(gomock.Any(), gomock.Any()).
			Return(&headlessv1.ListContactsResponse{
				Users: []*headlessv1.UserInfo{
					{Id: "U-friend1", Name: "Friend1", IconUrl: "https://example.test/icon1.png"},
					{Id: "U-friend2", Name: "Friend2", IconUrl: "https://example.test/icon2.png"},
				},
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListContactsRequest{
			HeadlessAccountId: "U-contact1",
			Limit:             50,
		})

		res, err := client.ListContacts(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetContacts(), 2)
		assert.Equal(t, "U-friend1", res.Msg.GetContacts()[0].GetId())
		assert.Equal(t, "Friend1", res.Msg.GetContacts()[0].GetName())
	})

	t.Run("成功: 最小権限 caller (account:read) で取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-lcontacts"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-lc-acc", "x@example.test", "p", groupID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-lc-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().ListContacts(gomock.Any(), gomock.Any()).Return(&headlessv1.ListContactsResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.ListContactsRequest{
			HeadlessAccountId: "U-mp-lc-acc",
			Limit:             10,
		}, "U-mp-lcontacts", groupID, []string{entity.PermKey_AccountRead})

		_, err := client.ListContacts(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: 起動中のホストがない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account without running host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-nohost", "nohost@example.test", "password")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListContactsRequest{
			HeadlessAccountId: "U-nohost",
			Limit:             50,
		})

		_, err := client.ListContacts(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_GetContactMessages(t *testing.T) {
	t.Run("成功: メッセージ履歴を取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-msg1", "msg@example.test", "password")
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-msg1", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to get contact messages
		setup.mockRpcClient.EXPECT().
			GetContactMessages(gomock.Any(), gomock.Any()).
			Return(&headlessv1.GetContactMessagesResponse{
				Messages: []*headlessv1.ContactChatMessage{
					{Id: "msg1", Type: headlessv1.ContactChatMessageType_CONTACT_CHAT_MESSAGE_TYPE_TEXT, Content: "Hello", SenderId: "U-friend"},
					{Id: "msg2", Type: headlessv1.ContactChatMessageType_CONTACT_CHAT_MESSAGE_TYPE_TEXT, Content: "Hi there", SenderId: "U-msg1"},
				},
				HasMoreBefore: true,
				HasMoreAfter:  false,
			}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetContactMessagesRequest{
			HeadlessAccountId: "U-msg1",
			ContactUserId:     "U-friend",
			Limit:             50,
		})

		res, err := client.GetContactMessages(t.Context(), req)
		require.NoError(t, err)
		assert.Len(t, res.Msg.GetMessages(), 2)
		assert.Equal(t, "Hello", res.Msg.GetMessages()[0].GetContent())
		assert.False(t, res.Msg.GetMessages()[0].GetIsOwnMessage()) // Message from friend
		assert.True(t, res.Msg.GetMessages()[1].GetIsOwnMessage())  // Message from self
		assert.True(t, res.Msg.GetHasMoreBefore())
		assert.False(t, res.Msg.GetHasMoreAfter())
	})

	t.Run("成功: 最小権限 caller (account:use) で取得", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-gmsgs"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-gm-acc", "x@example.test", "p", groupID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-gm-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().GetContactMessages(gomock.Any(), gomock.Any()).Return(&headlessv1.GetContactMessagesResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetContactMessagesRequest{
			HeadlessAccountId: "U-mp-gm-acc",
			ContactUserId:     "U-friend",
			Limit:             10,
		}, "U-mp-gmsgs", groupID, []string{entity.PermKey_AccountUse})

		_, err := client.GetContactMessages(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: account:read のみでは DM 履歴を取得できない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-gmsgs-ro"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-gmro-acc", "x@example.test", "p", groupID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-gmro-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.GetContactMessagesRequest{
			HeadlessAccountId: "U-mp-gmro-acc",
			ContactUserId:     "U-friend",
			Limit:             10,
		}, "U-mp-gmsgs-ro", groupID, []string{entity.PermKey_AccountRead})

		_, err := client.GetContactMessages(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
	})

	t.Run("失敗: 起動中のホストがない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account without running host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-nomsghost", "nomsghost@example.test", "password")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.GetContactMessagesRequest{
			HeadlessAccountId: "U-nomsghost",
			ContactUserId:     "U-friend",
			Limit:             50,
		})

		_, err := client.GetContactMessages(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}

func TestControllerService_SendContactMessage(t *testing.T) {
	t.Run("成功: メッセージを送信", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account and host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-send1", "send@example.test", "password")
		testutil.CreateTestHeadlessHost(t, setup.queries, "U-send1", "TestHost", entity.HeadlessHostStatus_RUNNING)

		// Mock HostConnector to return RPC client
		setup.mockHostConnector.EXPECT().
			GetRpcClient(gomock.Any(), gomock.Any()).
			Return(setup.mockRpcClient, nil)

		// Mock RPC call to send contact message
		setup.mockRpcClient.EXPECT().
			SendContactMessage(gomock.Any(), gomock.Any()).
			Return(&headlessv1.SendContactMessageResponse{}, nil)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SendContactMessageRequest{
			HeadlessAccountId: "U-send1",
			ContactUserId:     "U-friend",
			Message:           "Hello, friend!",
		})

		res, err := client.SendContactMessage(t.Context(), req)
		require.NoError(t, err)
		assert.NotNil(t, res.Msg)
	})

	t.Run("成功: 最小権限 caller (account:use) で送信", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		const groupID = "g-mp-send"
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-mp-send-acc", "x@example.test", "p", groupID)
		testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-mp-send-acc", "TestHost", entity.HeadlessHostStatus_RUNNING, groupID)

		setup.mockHostConnector.EXPECT().GetRpcClient(gomock.Any(), gomock.Any()).Return(setup.mockRpcClient, nil)
		setup.mockRpcClient.EXPECT().SendContactMessage(gomock.Any(), gomock.Any()).Return(&headlessv1.SendContactMessageResponse{}, nil)

		req := authAsMinPerm(t, setup.queries, &hdlctrlv1.SendContactMessageRequest{
			HeadlessAccountId: "U-mp-send-acc",
			ContactUserId:     "U-friend",
			Message:           "hi",
		}, "U-mp-send", groupID, []string{entity.PermKey_AccountUse})

		_, err := client.SendContactMessage(t.Context(), req)
		require.NoError(t, err)
	})

	t.Run("失敗: 起動中のホストがない", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		// Create test headless account without running host
		testutil.CreateTestHeadlessAccount(t, setup.queries, "U-nosendhost", "nosendhost@example.test", "password")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.SendContactMessageRequest{
			HeadlessAccountId: "U-nosendhost",
			ContactUserId:     "U-friend",
			Message:           "Hello!",
		})

		_, err := client.SendContactMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		ok := errors.As(err, &connectErr)
		require.True(t, ok, "expected connect.Error")
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})
}
