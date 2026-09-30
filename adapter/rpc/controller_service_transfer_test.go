package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transferAllWritePerms は移管単位 (アカウント + ホスト + セッション) を丸ごと移すのに
// 移管元・移管先の両方で必要になる permission.
var transferAllWritePerms = []string{
	entity.PermKey_AccountRead,
	entity.PermKey_AccountWrite,
	entity.PermKey_AccountUse,
	entity.PermKey_HostRead,
	entity.PermKey_HostWrite,
	entity.PermKey_SessionRead,
	entity.PermKey_SessionWrite,
}

func transferHostReq(hostID, destinationGroupID string, dryRun bool) *hdlctrlv1.TransferResourcesRequest {
	return &hdlctrlv1.TransferResourcesRequest{
		Resource:           &hdlctrlv1.TransferResourcesRequest_HostId{HostId: hostID},
		DestinationGroupId: destinationGroupID,
		DryRun:             dryRun,
	}
}

func transferredIDs[T interface{ GetId() string }](items []T) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.GetId())
	}

	return ids
}

func requireHostGroup(t *testing.T, queries *db.Queries, hostID, expectedGroupID string) {
	t.Helper()

	host, err := queries.GetHost(t.Context(), hostID)
	require.NoError(t, err)
	assert.Equal(t, expectedGroupID, host.GroupID)
}

func requireSessionGroup(t *testing.T, queries *db.Queries, sessionID, expectedGroupID string) {
	t.Helper()

	session, err := queries.GetSession(t.Context(), sessionID)
	require.NoError(t, err)
	assert.Equal(t, expectedGroupID, session.GroupID)
}

func TestControllerService_TransferResources(t *testing.T) {
	const (
		srcGroupID = "g-transfer-src"
		dstGroupID = "g-transfer-dst"
		accountID  = "U-transfer-acc"
	)

	t.Run("成功: ホスト起点で依存リソース一式 (アカウント / 同アカウントの全ホスト / セッション) を移管", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "src@example.test", "p", srcGroupID)
		host1 := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "Host1", entity.HeadlessHostStatus_RUNNING, srcGroupID)
		host2 := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "Host2", entity.HeadlessHostStatus_EXITED, srcGroupID)
		session1 := testutil.CreateTestSessionInGroup(t, setup.queries, host1.ID, "Session1", entity.SessionStatus_RUNNING, srcGroupID)
		session2 := testutil.CreateTestSessionInGroup(t, setup.queries, host2.ID, "Session2", entity.SessionStatus_ENDED, srcGroupID)

		// 移管元グループに残るべき、別アカウントのリソース.
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, "U-transfer-stay", "stay@example.test", "p", srcGroupID)
		stayHost := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, "U-transfer-stay", "StayHost", entity.HeadlessHostStatus_RUNNING, srcGroupID)
		staySession := testutil.CreateTestSessionInGroup(t, setup.queries, stayHost.ID, "StaySession", entity.SessionStatus_RUNNING, srcGroupID)

		const callerID = "U-transfer-caller"
		testutil.SetupUserWithExactPermissions(t, setup.queries, callerID, srcGroupID, transferAllWritePerms)
		testutil.AddUserToGroupWithExactPermissions(t, setup.queries, callerID, dstGroupID, transferAllWritePerms)

		// dry run は移管対象を返すだけで何も変更しない.
		dryRes, err := client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t,
			transferHostReq(host1.ID, dstGroupID, true), callerID, "U-resonite-"+callerID, ""))
		require.NoError(t, err)
		assert.Equal(t, srcGroupID, dryRes.Msg.GetSourceGroupId())
		assert.Equal(t, dstGroupID, dryRes.Msg.GetDestinationGroupId())
		assert.Equal(t, accountID, dryRes.Msg.GetAccount().GetUserId())
		assert.False(t, dryRes.Msg.GetAccount().GetMerged())
		assert.ElementsMatch(t, []string{host1.ID, host2.ID}, transferredIDs(dryRes.Msg.GetHosts()))
		assert.ElementsMatch(t, []string{session1.ID, session2.ID}, transferredIDs(dryRes.Msg.GetSessions()))
		requireHostGroup(t, setup.queries, host1.ID, srcGroupID)
		assert.Equal(t, srcGroupID, testutil.GetOnlyHeadlessAccount(t, setup.queries, accountID).GroupID)

		res, err := client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t,
			transferHostReq(host1.ID, dstGroupID, false), callerID, "U-resonite-"+callerID, ""))
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{host1.ID, host2.ID}, transferredIDs(res.Msg.GetHosts()))
		assert.ElementsMatch(t, []string{session1.ID, session2.ID}, transferredIDs(res.Msg.GetSessions()))

		assert.Equal(t, dstGroupID, testutil.GetOnlyHeadlessAccount(t, setup.queries, accountID).GroupID)
		requireHostGroup(t, setup.queries, host1.ID, dstGroupID)
		requireHostGroup(t, setup.queries, host2.ID, dstGroupID)
		requireSessionGroup(t, setup.queries, session1.ID, dstGroupID)
		requireSessionGroup(t, setup.queries, session2.ID, dstGroupID)

		// 別アカウントのリソースは移管元に残る.
		assert.Equal(t, srcGroupID, testutil.GetOnlyHeadlessAccount(t, setup.queries, "U-transfer-stay").GroupID)
		requireHostGroup(t, setup.queries, stayHost.ID, srcGroupID)
		requireSessionGroup(t, setup.queries, staySession.ID, srcGroupID)
	})

	t.Run("成功: 移管先に同一アカウントが登録済みなら移管先の登録へマージ", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "src@example.test", "src-pass", srcGroupID)
		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "dst@example.test", "dst-pass", dstGroupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "Host", entity.HeadlessHostStatus_RUNNING, srcGroupID)
		dstHost := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "DstHost", entity.HeadlessHostStatus_RUNNING, dstGroupID)

		// セッション起点でも同じ移管単位になる.
		session := testutil.CreateTestSessionInGroup(t, setup.queries, host.ID, "Session", entity.SessionStatus_RUNNING, srcGroupID)

		res, err := client.TransferResources(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.TransferResourcesRequest{
			Resource:           &hdlctrlv1.TransferResourcesRequest_SessionId{SessionId: session.ID},
			DestinationGroupId: dstGroupID,
		}))
		require.NoError(t, err)
		assert.True(t, res.Msg.GetAccount().GetMerged())
		// 移管先に元からあるホストは移管対象に含まれない.
		assert.Equal(t, []string{host.ID}, transferredIDs(res.Msg.GetHosts()))
		assert.Equal(t, []string{session.ID}, transferredIDs(res.Msg.GetSessions()))

		// 移管先の登録 (認証情報) が残り、移管元の登録は消える.
		merged := testutil.GetOnlyHeadlessAccount(t, setup.queries, accountID)
		assert.Equal(t, dstGroupID, merged.GroupID)
		assert.Equal(t, "dst@example.test", merged.Credential)
		assert.Equal(t, "dst-pass", merged.Password)

		requireHostGroup(t, setup.queries, host.ID, dstGroupID)
		requireHostGroup(t, setup.queries, dstHost.ID, dstGroupID)
		requireSessionGroup(t, setup.queries, session.ID, dstGroupID)
	})

	t.Run("成功: アカウント起点 (ホスト無し) は account:write だけで移管できる", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "src@example.test", "p", srcGroupID)

		const callerID = "U-transfer-acc-only"

		perms := []string{entity.PermKey_AccountRead, entity.PermKey_AccountWrite}
		testutil.SetupUserWithExactPermissions(t, setup.queries, callerID, srcGroupID, perms)
		testutil.AddUserToGroupWithExactPermissions(t, setup.queries, callerID, dstGroupID, perms)

		res, err := client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.TransferResourcesRequest{
			Resource: &hdlctrlv1.TransferResourcesRequest_Account{
				Account: &hdlctrlv1.HeadlessAccountRef{GroupId: srcGroupID, AccountId: accountID},
			},
			DestinationGroupId: dstGroupID,
		}, callerID, "U-resonite-"+callerID, ""))
		require.NoError(t, err)
		assert.Equal(t, accountID, res.Msg.GetAccount().GetUserId())
		assert.Empty(t, res.Msg.GetHosts())
		assert.Empty(t, res.Msg.GetSessions())

		assert.Equal(t, dstGroupID, testutil.GetOnlyHeadlessAccount(t, setup.queries, accountID).GroupID)
	})

	t.Run("成功: ホスト削除済みのセッションは単体で移管", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestGroup(t, setup.queries, dstGroupID, "")
		session := testutil.CreateTestSessionInGroup(t, setup.queries, "deleted-host", "Orphan", entity.SessionStatus_ENDED, srcGroupID)

		res, err := client.TransferResources(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, &hdlctrlv1.TransferResourcesRequest{
			Resource:           &hdlctrlv1.TransferResourcesRequest_SessionId{SessionId: session.ID},
			DestinationGroupId: dstGroupID,
		}))
		require.NoError(t, err)
		assert.Nil(t, res.Msg.GetAccount())
		assert.Empty(t, res.Msg.GetHosts())
		assert.Equal(t, []string{session.ID}, transferredIDs(res.Msg.GetSessions()))

		requireSessionGroup(t, setup.queries, session.ID, dstGroupID)
	})

	t.Run("失敗: 権限不足 (移管先 / 移管元 / 閲覧不可)", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "src@example.test", "p", srcGroupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "Host", entity.HeadlessHostStatus_RUNNING, srcGroupID)
		testutil.CreateTestSessionInGroup(t, setup.queries, host.ID, "Session", entity.SessionStatus_RUNNING, srcGroupID)

		// 移管先でセッションを書けない (session:write 不足).
		const noDstPerm = "U-transfer-no-dst"
		testutil.SetupUserWithExactPermissions(t, setup.queries, noDstPerm, srcGroupID, transferAllWritePerms)
		testutil.AddUserToGroupWithExactPermissions(t, setup.queries, noDstPerm, dstGroupID, []string{
			entity.PermKey_AccountWrite, entity.PermKey_AccountUse, entity.PermKey_HostWrite,
		})

		_, err := client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t,
			transferHostReq(host.ID, dstGroupID, false), noDstPerm, "U-resonite-"+noDstPerm, ""))
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

		// 移管元でアカウントを書けない (ホストだけ持ち出してアカウントを道連れにはできない).
		const noSrcPerm = "U-transfer-no-src"
		testutil.SetupUserWithExactPermissions(t, setup.queries, noSrcPerm, srcGroupID, []string{
			entity.PermKey_HostRead, entity.PermKey_HostWrite, entity.PermKey_SessionWrite,
		})
		testutil.AddUserToGroupWithExactPermissions(t, setup.queries, noSrcPerm, dstGroupID, transferAllWritePerms)

		_, err = client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t,
			transferHostReq(host.ID, dstGroupID, true), noSrcPerm, "U-resonite-"+noSrcPerm, ""))
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

		// 起点リソースを閲覧できない caller には存在を明かさない.
		const outsider = "U-transfer-outsider"
		testutil.SetupUserWithExactPermissions(t, setup.queries, outsider, dstGroupID+"-outsider", transferAllWritePerms)

		_, err = client.TransferResources(t.Context(), testutil.CreateAuthenticatedRequest(t,
			transferHostReq(host.ID, dstGroupID+"-outsider", false), outsider, "U-resonite-"+outsider, ""))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

		// いずれの失敗でも何も移っていない.
		requireHostGroup(t, setup.queries, host.ID, srcGroupID)
		assert.Equal(t, srcGroupID, testutil.GetOnlyHeadlessAccount(t, setup.queries, accountID).GroupID)
	})

	t.Run("失敗: 移管先にできないグループ / 起点未指定", func(t *testing.T) {
		setup := setupControllerServiceTest(t)
		defer setup.Cleanup()

		client := setupAuthenticatedClient(t, setup.service)

		testutil.CreateTestHeadlessAccountInGroup(t, setup.queries, accountID, "src@example.test", "p", srcGroupID)
		host := testutil.CreateTestHeadlessHostInGroup(t, setup.queries, accountID, "Host", entity.HeadlessHostStatus_RUNNING, srcGroupID)

		for name, tc := range map[string]struct {
			req  *hdlctrlv1.TransferResourcesRequest
			code connect.Code
		}{
			"移管元と同じグループ": {transferHostReq(host.ID, srcGroupID, false), connect.CodeFailedPrecondition},
			"system グループ": {transferHostReq(host.ID, entity.SystemGroupID, false), connect.CodeFailedPrecondition},
			"移管先未指定":      {transferHostReq(host.ID, "", false), connect.CodeFailedPrecondition},
			"存在しないグループ":  {transferHostReq(host.ID, "g-transfer-missing", false), connect.CodeNotFound},
			"存在しないホスト":   {transferHostReq("missing-host", dstGroupID, false), connect.CodeNotFound},
			"起点未指定":       {&hdlctrlv1.TransferResourcesRequest{DestinationGroupId: dstGroupID}, connect.CodeInvalidArgument},
		} {
			_, err := client.TransferResources(t.Context(), testutil.CreateDefaultAuthenticatedRequest(t, tc.req))
			require.Error(t, err, name)
			assert.Equal(t, tc.code, connect.CodeOf(err), name)
		}

		requireHostGroup(t, setup.queries, host.ID, srcGroupID)
	})
}
