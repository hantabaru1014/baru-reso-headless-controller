package rpc

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissionInterceptor_AllProceduresRegistered は全ての既知 procedure に
// permission ルールが登録されていることを直接 assert する.
// registerRPCPermission が init() より先に動くため、ここに到達した時点で
// rpcPermissionRules は完成している.
func TestPermissionInterceptor_AllProceduresRegistered(t *testing.T) {
	for _, p := range allKnownProcedures() {
		if _, ok := rpcPermissionRules[p]; !ok {
			t.Errorf("procedure %q is not registered in rpcPermissionRules", p)
		}
	}
}

// TestPermissionInterceptor_DeniesUserWithoutPermission verifies that a user
// who is only a member of a custom group without write permission cannot
// invoke host-write operations.
func TestPermissionInterceptor_DeniesUserWithoutPermission(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	const callerUserID = "U-no-permission"

	// 1) caller を作成 (system-admin にはしない).
	testutil.CreateTestUser(t, setup.queries, callerUserID, "dummy")

	// 2) host/account を migrated-pre-permission グループに置く (fixture デフォルト).
	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-account", "user@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-account", "TestHost", entity.HeadlessHostStatus_EXITED)

	// 3) caller を migrated-pre-permission グループに session-operator として登録
	//    (host:read / host:use はあるが host:write は無い).
	_, err := setup.queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: entity.MigratedPrePermissionGroupID,
		UserID:  callerUserID,
		RoleID:  entity.SeedRoleID_SessionOperator,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err)

	client := setupAuthenticatedClient(t, setup.service)

	// host:write が必要な ShutdownHeadlessHost は PermissionDenied.
	req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
		HostId: host.ID,
	}, callerUserID, "U-resonite", "")

	_, err = client.ShutdownHeadlessHost(t.Context(), req)
	require.Error(t, err)

	connectErr := &connect.Error{}
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())
}

// TestPermissionInterceptor_AllowsUserWithSystemGroupManage verifies that a
// user with system:group.manage can act on a host they are not directly a
// member of (system override).
func TestPermissionInterceptor_AllowsUserWithSystemGroupManage(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	const callerUserID = "U-system-admin"

	// caller は system-admin (system グループメンバー).
	testutil.SetupSystemAdminUser(t, setup.queries, callerUserID)

	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-account", "user@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-account", "TestHost", entity.HeadlessHostStatus_EXITED)

	client := setupAuthenticatedClient(t, setup.service)

	req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
		HostId: host.ID,
	}, callerUserID, "U-resonite", "")

	res, err := client.ShutdownHeadlessHost(t.Context(), req)
	require.NoError(t, err)
	require.NotNil(t, res.Msg)
	assert.NotEmpty(t, res.Msg.GetJobId(), "system-admin should be able to shutdown via system:group.manage override")
}

// TestPermissionInterceptor_DeniesNonMemberOfGroup verifies that a user who
// is not a member of any group cannot access protected operations.
func TestPermissionInterceptor_DeniesNonMemberOfGroup(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	const callerUserID = "U-stranger"

	testutil.CreateTestUser(t, setup.queries, callerUserID, "dummy")

	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-account", "user@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-account", "TestHost", entity.HeadlessHostStatus_EXITED)

	client := setupAuthenticatedClient(t, setup.service)

	req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
		HostId: host.ID,
	}, callerUserID, "U-resonite", "")

	_, err := client.GetHeadlessHost(t.Context(), req)
	require.Error(t, err)

	connectErr := &connect.Error{}
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
		"non-member should get PermissionDenied on protected RPC")
}
