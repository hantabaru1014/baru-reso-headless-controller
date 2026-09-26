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
// is not a member of any group cannot access protected operations, and that
// the error is indistinguishable from the one for a non-existent resource
// (閲覧権限の無い caller に ID の存在有無を判別させない).
func TestPermissionInterceptor_DeniesNonMemberOfGroup(t *testing.T) {
	setup := setupControllerServiceTest(t)
	defer setup.Cleanup()

	const callerUserID = "U-stranger"

	testutil.CreateTestUser(t, setup.queries, callerUserID, "dummy")

	testutil.CreateTestHeadlessAccount(t, setup.queries, "U-account", "user@example.test", "password")
	host := testutil.CreateTestHeadlessHost(t, setup.queries, "U-account", "TestHost", entity.HeadlessHostStatus_EXITED)
	session := testutil.CreateTestSession(t, setup.queries, host.ID, "TestSession", entity.SessionStatus_ENDED)

	client := setupAuthenticatedClient(t, setup.service)

	getHost := func(hostID string) *connect.Error {
		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.GetHeadlessHostRequest{
			HostId: hostID,
		}, callerUserID, "U-resonite", "")

		_, err := client.GetHeadlessHost(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)

		return connectErr
	}

	hostErr := getHost(host.ID)
	missingHostErr := getHost("non-existent-host")

	assert.Equal(t, connect.CodeNotFound, hostErr.Code(),
		"non-member should get NotFound on host they cannot read")
	assert.Equal(t, missingHostErr.Code(), hostErr.Code())
	assert.Equal(t, missingHostErr.Message(), hostErr.Message())

	// host:read の無い write 系 RPC も存在を漏らさない.
	shutdownReq := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.ShutdownHeadlessHostRequest{
		HostId: host.ID,
	}, callerUserID, "U-resonite", "")
	_, err := client.ShutdownHeadlessHost(t.Context(), shutdownReq)
	require.Error(t, err)

	shutdownErr := &connect.Error{}
	require.ErrorAs(t, err, &shutdownErr)
	assert.Equal(t, connect.CodeNotFound, shutdownErr.Code())

	getSession := func(sessionID string) *connect.Error {
		req := testutil.CreateAuthenticatedRequest(t, &hdlctrlv1.GetSessionDetailsRequest{
			SessionId: sessionID,
		}, callerUserID, "U-resonite", "")

		_, err := client.GetSessionDetails(t.Context(), req)
		require.Error(t, err)

		connectErr := &connect.Error{}
		require.ErrorAs(t, err, &connectErr)

		return connectErr
	}

	sessionErr := getSession(session.ID)
	missingSessionErr := getSession("non-existent-session")

	assert.Equal(t, connect.CodeNotFound, sessionErr.Code(),
		"non-member should get NotFound on session they cannot read")
	assert.Equal(t, missingSessionErr.Code(), sessionErr.Code())
	assert.Equal(t, missingSessionErr.Message(), sessionErr.Message())
}
