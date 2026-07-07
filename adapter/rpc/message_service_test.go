package rpc

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1/hdlctrlv1connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

type messageServiceTestSetup struct {
	service *MessageService
	queries *db.Queries
	pool    *pgxpool.Pool
}

func setupMessageServiceTest(t *testing.T) *messageServiceTestSetup {
	t.Helper()

	auth.Init("test-jwt-secret-for-testing")

	queries, pool := testutil.SetupTestDB(t)
	testutil.CleanupTables(t, pool)

	// デフォルト test user (`test@example.test`) を system-admin にする.
	// seed-system-admin は system:message.manage を持つため、この user が
	// 「管理権限保持者」の呼び出し元になる.
	testutil.SetupDefaultSystemAdminUser(t, queries)

	groupRepo := adapter.NewGroupRepository(queries)
	roleRepo := adapter.NewRoleRepository(queries)
	permUC := newPermissionUsecaseForTest(queries)
	muc := usecase.NewMessageUsecase(adapter.NewMessageRepository(queries), groupRepo, permUC)

	// host/session/account 依存は MessageService の permission ルール
	// (requireAuthenticated / requireSystemPerm) では参照されないため nil で良い.
	service := NewMessageService(muc, permUC, nil, nil, nil, groupRepo, roleRepo)

	return &messageServiceTestSetup{
		service: service,
		queries: queries,
		pool:    pool,
	}
}

func setupMessageServiceClient(t *testing.T, service *MessageService) hdlctrlv1connect.MessageServiceClient {
	t.Helper()

	server := testutil.SetupAuthenticatedHTTPServer(t, service)
	t.Cleanup(server.Close)

	return hdlctrlv1connect.NewMessageServiceClient(server.Client(), server.URL)
}

func TestMessageService_ListMessages(t *testing.T) {
	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		_, err := client.ListMessages(t.Context(), connect.NewRequest(&hdlctrlv1.ListMessagesRequest{}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("成功: 一般ユーザーは全員向け + 所属グループ向けのみ (updated_at 降順)", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		// alice は group-a に所属し、group-b には所属しない.
		testutil.CreateTestUser(t, setup.queries, "alice@example.test", "dummy-password")
		testutil.CreateTestGroup(t, setup.queries, "group-a", "alice@example.test")
		testutil.CreateTestGroup(t, setup.queries, "group-b", "")

		base := time.Now()
		groupA := "group-a"
		groupB := "group-b"

		// updated_at 降順で [global, group-a, group-b] になるよう timestamp を設定.
		testutil.CreateTestMessage(t, setup.pool, "msg-global", "全員向け", "body", nil, "test@example.test", base.Add(3*time.Second))
		testutil.CreateTestMessage(t, setup.pool, "msg-a", "group-a向け", "body", &groupA, "test@example.test", base.Add(2*time.Second))
		testutil.CreateTestMessage(t, setup.pool, "msg-b", "group-b向け", "body", &groupB, "test@example.test", base.Add(1*time.Second))

		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ListMessagesRequest{}, "alice@example.test", "U-alice", "")
		res, err := client.ListMessages(t.Context(), req)
		require.NoError(t, err)

		got := res.Msg.GetMessages()
		require.Len(t, got, 2, "group-b向けは見えないはず")
		assert.Equal(t, "msg-global", got[0].GetId(), "updated_at 降順の先頭は全員向け")
		assert.Equal(t, "msg-a", got[1].GetId())
		// group_name が解決されている.
		assert.Equal(t, "group-a", got[1].GetGroupName())
		assert.Empty(t, got[0].GetGroupId(), "全員向けは group_id が空")
	})

	t.Run("成功: system:message.manage 保持者は全件見える (updated_at 降順)", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestGroup(t, setup.queries, "group-a", "")

		base := time.Now()
		groupA := "group-a"

		testutil.CreateTestMessage(t, setup.pool, "msg-global", "全員向け", "body", nil, "test@example.test", base.Add(2*time.Second))
		testutil.CreateTestMessage(t, setup.pool, "msg-a", "group-a向け", "body", &groupA, "test@example.test", base.Add(1*time.Second))

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.ListMessagesRequest{})
		res, err := client.ListMessages(t.Context(), req)
		require.NoError(t, err)

		got := res.Msg.GetMessages()
		require.Len(t, got, 2)
		assert.Equal(t, "msg-global", got[0].GetId())
		assert.Equal(t, "msg-a", got[1].GetId())
	})
}

func TestMessageService_CreateMessage(t *testing.T) {
	t.Run("成功: system:message.manage 保持者が全員向けメッセージを作成", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateMessageRequest{
			Title: "お知らせ",
			Body:  "# 見出し\n本文",
		})
		res, err := client.CreateMessage(t.Context(), req)
		require.NoError(t, err)

		msg := res.Msg.GetMessage()
		assert.NotEmpty(t, msg.GetId())
		assert.Equal(t, "お知らせ", msg.GetTitle())
		assert.Equal(t, "# 見出し\n本文", msg.GetBody())
		assert.Empty(t, msg.GetGroupId(), "全員向けは group_id が空")
		assert.Equal(t, "test@example.test", msg.GetCreatedBy())
		assert.Equal(t, "test@example.test", msg.GetLastUpdatedBy())

		// 実際に DB に保存されている.
		stored, err := setup.queries.GetMessage(t.Context(), msg.GetId())
		require.NoError(t, err)
		assert.Equal(t, "お知らせ", stored.Message.Title)
	})

	t.Run("成功: グループ向けメッセージを作成 (group_name が解決される)", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestGroup(t, setup.queries, "group-a", "")

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateMessageRequest{
			Title:   "グループ向け",
			Body:    "body",
			GroupId: proto.String("group-a"),
		})
		res, err := client.CreateMessage(t.Context(), req)
		require.NoError(t, err)

		msg := res.Msg.GetMessage()
		assert.Equal(t, "group-a", msg.GetGroupId())
		assert.Equal(t, "group-a", msg.GetGroupName())
	})

	t.Run("失敗: title 空 → InvalidArgument", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateMessageRequest{
			Title: "",
			Body:  "body",
		})
		_, err := client.CreateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: 存在しないグループ → NotFound", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.CreateMessageRequest{
			Title:   "t",
			Body:    "body",
			GroupId: proto.String("ghost-group"),
		})
		_, err := client.CreateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: 認証なし → Unauthenticated", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		_, err := client.CreateMessage(t.Context(), connect.NewRequest(&hdlctrlv1.CreateMessageRequest{
			Title: "t",
			Body:  "body",
		}))
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())
	})

	t.Run("失敗: system:message.manage 権限なし → PermissionDenied", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		// system 権限を持たない一般ユーザー.
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, "alice@example.test")

		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.CreateMessageRequest{
			Title: "t",
			Body:  "body",
		}, "alice@example.test", "U-alice", "")
		_, err := client.CreateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
		assert.Contains(t, connectErr.Message(), entity.PermKey_SystemMessageManage)
	})
}

func TestMessageService_UpdateMessage(t *testing.T) {
	t.Run("成功: group_id を全員向け⇔グループ向けに変更できる", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestGroup(t, setup.queries, "group-a", "")
		testutil.CreateTestMessage(t, setup.pool, "msg-1", "元タイトル", "元本文", nil, "test@example.test", time.Now())

		// 全員向け → グループ向け.
		toGroup := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateMessageRequest{
			MessageId: "msg-1",
			Title:     "更新後",
			Body:      "更新本文",
			GroupId:   proto.String("group-a"),
		})
		res, err := client.UpdateMessage(t.Context(), toGroup)
		require.NoError(t, err)
		assert.Equal(t, "更新後", res.Msg.GetMessage().GetTitle())
		assert.Equal(t, "group-a", res.Msg.GetMessage().GetGroupId())
		assert.Equal(t, "group-a", res.Msg.GetMessage().GetGroupName())

		// グループ向け → 全員向け (group_id 未指定).
		toAll := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateMessageRequest{
			MessageId: "msg-1",
			Title:     "全員向けに戻す",
			Body:      "body",
		})
		res, err = client.UpdateMessage(t.Context(), toAll)
		require.NoError(t, err)
		assert.Empty(t, res.Msg.GetMessage().GetGroupId(), "全員向けに戻ると group_id は空")
		assert.Equal(t, "全員向けに戻す", res.Msg.GetMessage().GetTitle())
	})

	t.Run("失敗: title 空 → InvalidArgument", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestMessage(t, setup.pool, "msg-1", "t", "body", nil, "test@example.test", time.Now())

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateMessageRequest{
			MessageId: "msg-1",
			Title:     "",
			Body:      "body",
		})
		_, err := client.UpdateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	})

	t.Run("失敗: 存在しないメッセージ → NotFound", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.UpdateMessageRequest{
			MessageId: "msg-ghost",
			Title:     "t",
			Body:      "body",
		})
		_, err := client.UpdateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: system:message.manage 権限なし → PermissionDenied", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestMessage(t, setup.pool, "msg-1", "t", "body", nil, "test@example.test", time.Now())
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, "alice@example.test")

		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.UpdateMessageRequest{
			MessageId: "msg-1",
			Title:     "t2",
			Body:      "body",
		}, "alice@example.test", "U-alice", "")
		_, err := client.UpdateMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
	})
}

func TestMessageService_DeleteMessage(t *testing.T) {
	t.Run("成功: system:message.manage 保持者が削除", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestMessage(t, setup.pool, "msg-1", "t", "body", nil, "test@example.test", time.Now())

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteMessageRequest{MessageId: "msg-1"})
		_, err := client.DeleteMessage(t.Context(), req)
		require.NoError(t, err)

		// 実際に削除されている.
		_, err = setup.queries.GetMessage(t.Context(), "msg-1")
		require.Error(t, err)
	})

	t.Run("失敗: 存在しないメッセージ → NotFound", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		req := testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.DeleteMessageRequest{MessageId: "msg-ghost"})
		_, err := client.DeleteMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodeNotFound, connectErr.Code())
	})

	t.Run("失敗: system:message.manage 権限なし → PermissionDenied", func(t *testing.T) {
		setup := setupMessageServiceTest(t)
		client := setupMessageServiceClient(t, setup.service)

		testutil.CreateTestMessage(t, setup.pool, "msg-1", "t", "body", nil, "test@example.test", time.Now())
		testutil.SetupNormalUserWithPersonalGroup(t, setup.queries, "alice@example.test")

		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.DeleteMessageRequest{MessageId: "msg-1"},
			"alice@example.test", "U-alice", "")
		_, err := client.DeleteMessage(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)
		assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

		// msg-1 は残っている.
		_, err = setup.queries.GetMessage(t.Context(), "msg-1")
		require.NoError(t, err)
	})
}
