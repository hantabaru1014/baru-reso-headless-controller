package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/google/uuid"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/encoding/protojson"
)

// CreateTestUser creates a test user in the database.
func CreateTestUser(t *testing.T, queries *db.Queries, id, password string) db.User {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	resoniteID := pgtype.Text{String: "U-test-user-" + uuid.New().String(), Valid: true}
	iconURL := pgtype.Text{String: "https://example.com/icon.png", Valid: true}

	err = queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:         id,
		Password:   string(hashedPassword),
		ResoniteID: resoniteID,
		IconUrl:    iconURL,
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	user, err := queries.GetUser(t.Context(), id)
	if err != nil {
		t.Fatalf("failed to get created user: %v", err)
	}

	return user
}

// CreateTestHeadlessAccount creates a test headless account in the database.
func CreateTestHeadlessAccount(t *testing.T, queries *db.Queries, resoniteID, credential, password string) db.HeadlessAccount {
	t.Helper()

	displayName := pgtype.Text{String: "Test Headless Account", Valid: true}
	iconURL := pgtype.Text{String: "https://example.com/headless-icon.png", Valid: true}

	err := queries.CreateHeadlessAccount(t.Context(), db.CreateHeadlessAccountParams{
		ResoniteID:      resoniteID,
		Credential:      credential,
		Password:        password,
		LastDisplayName: displayName,
		LastIconUrl:     iconURL,
		GroupID:         entity.MigratedPrePermissionGroupID,
		CreatedBy:       pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test headless account: %v", err)
	}

	account, err := queries.GetHeadlessAccount(t.Context(), resoniteID)
	if err != nil {
		t.Fatalf("failed to get created headless account: %v", err)
	}

	return account
}

// CreateTestHeadlessHost creates a test headless host in the database.
func CreateTestHeadlessHost(t *testing.T, queries *db.Queries, accountID, name string, status entity.HeadlessHostStatus) db.Host {
	t.Helper()

	id := uniuri.New()
	startupConfig := []byte(`{"tickRate": 60.0}`)
	connectString := "test-container-" + id

	host, err := queries.CreateHost(t.Context(), db.CreateHostParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		AccountID:                      accountID,
		CreatedBy:                      pgtype.Text{Valid: false},
		GroupID:                        entity.MigratedPrePermissionGroupID,
		LastStartupConfig:              startupConfig,
		LastStartupConfigSchemaVersion: 1,
		ConnectorType:                  "docker",
		ConnectString:                  connectString,
		AutoUpdatePolicy:               int32(entity.HostAutoUpdatePolicy_UNSPECIFIED),
		Memo:                           pgtype.Text{Valid: false},
		InstanceCount:                  1,
		StartedAt: pgtype.Timestamptz{
			Valid: true,
			Time:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test headless host: %v", err)
	}

	return host
}

// CreateTestSession creates a test session in the database.
func CreateTestSession(t *testing.T, queries *db.Queries, hostID, name string, status entity.SessionStatus) db.Session {
	t.Helper()

	return CreateTestSessionWithStartupParameters(t, queries, hostID, name, status, nil)
}

// CreateTestSessionWithStartupParameters creates a test session with explicit
// startup_parameters JSONB, simulating various pre-existing DB state in tests.
func CreateTestSessionWithStartupParameters(t *testing.T, queries *db.Queries, hostID, name string, status entity.SessionStatus, startupParameters *headlessv1.WorldStartupParameters) db.Session {
	t.Helper()

	id := uniuri.New()
	startupParams := []byte(`{"maxUsers": 8}`)

	if startupParameters != nil {
		b, err := protojson.Marshal(startupParameters)
		if err != nil {
			t.Fatalf("failed to marshal startup_parameters: %v", err)
		}

		startupParams = b
	}

	session, err := queries.UpsertSession(t.Context(), db.UpsertSessionParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		StartedAt:                      pgtype.Timestamptz{Valid: true, Time: time.Now()},
		CreatedBy:                      pgtype.Text{Valid: false},
		GroupID:                        entity.MigratedPrePermissionGroupID,
		EndedAt:                        pgtype.Timestamptz{Valid: false},
		HostID:                         hostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    false,
		Memo:                           pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	return session
}

// SetupSystemAdminUser は test user を作成し system グループに system-admin ロールで
// 登録する. permission interceptor を持つ RPC テストで使う.
//
// 通常テストは `test@example.test` を呼び出しユーザーとするため、デフォルトの
// `SetupDefaultSystemAdminUser` をテスト setup から呼べば全 permission チェックが通る.
//
// CreateHeadlessAccount 等が呼び出しユーザーの personal group を resolve しに行くため
// 同時に personal group も投入する.
func SetupSystemAdminUser(t *testing.T, queries *db.Queries, userID string) {
	t.Helper()

	_ = CreateTestUser(t, queries, userID, "dummy-password")

	_, err := queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: entity.SystemGroupID,
		UserID:  userID,
		RoleID:  entity.SeedRoleID_SystemAdmin,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add system-admin member")

	personalGroupID := userID + "-personal"

	_, err = queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   personalGroupID,
		Name: personalGroupID,
		Type: string(entity.GroupType_Personal),
	})
	require.NoError(t, err, "failed to create personal group")

	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: personalGroupID,
		UserID:  userID,
		RoleID:  entity.SeedRoleID_Admin,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add personal group member")
}

// SetupDefaultSystemAdminUser は CreateDefaultAuthenticatedRequest が使うデフォルト
// userID (`test@example.test`) を system-admin として bootstrap する.
func SetupDefaultSystemAdminUser(t *testing.T, queries *db.Queries) {
	t.Helper()
	SetupSystemAdminUser(t, queries, "test@example.test")
}

// SetupNormalUserWithPersonalGroup は user を作成し personal グループのみに
// `seed-admin` として登録する (system 非所属). List 系 RPC のグループフィルタ /
// 認可テストで「特定グループだけにアクセス可能なユーザー」を用意するために使う.
//
// 戻り値は作成された personal グループの ID (`<userID>-personal`).
func SetupNormalUserWithPersonalGroup(t *testing.T, queries *db.Queries, userID string) string {
	t.Helper()

	_ = CreateTestUser(t, queries, userID, "dummy-password")

	personalGroupID := userID + "-personal"

	_, err := queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   personalGroupID,
		Name: personalGroupID,
		Type: string(entity.GroupType_Personal),
	})
	require.NoError(t, err, "failed to create personal group")

	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: personalGroupID,
		UserID:  userID,
		RoleID:  entity.SeedRoleID_Admin,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add personal group member")

	return personalGroupID
}

// CreateTestGroup は normal グループを作成し、指定 user を seed-admin として登録する.
// List 系 RPC テストで「複数グループ × 複数ユーザー」のシナリオを組むために使う.
func CreateTestGroup(t *testing.T, queries *db.Queries, groupID, ownerUserID string) {
	t.Helper()

	_, err := queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   groupID,
		Name: groupID,
		Type: string(entity.GroupType_Normal),
	})
	require.NoError(t, err, "failed to create normal group")

	if ownerUserID != "" {
		_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
			GroupID: groupID,
			UserID:  ownerUserID,
			RoleID:  entity.SeedRoleID_Admin,
			AddedBy: pgtype.Text{Valid: false},
		})
		require.NoError(t, err, "failed to add owner as admin")
	}
}

// ensureNormalGroupExists は指定 groupID の normal グループが存在しなければ作成する.
// `*InGroup` ヘルパーで暗黙に呼び、テスト記述から「グループ作成」のボイラープレート
// を削るためのもの.
func ensureNormalGroupExists(t *testing.T, queries *db.Queries, groupID string) {
	t.Helper()

	if groupID == "" || groupID == entity.MigratedPrePermissionGroupID || groupID == entity.SystemGroupID {
		return
	}

	if _, err := queries.GetGroup(t.Context(), groupID); err == nil {
		return
	}

	_, err := queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   groupID,
		Name: groupID,
		Type: string(entity.GroupType_Normal),
	})
	require.NoError(t, err, "failed to auto-create normal group %q", groupID)
}

// CreateTestHeadlessHostInGroup は CreateTestHeadlessHost のグループ指定版.
func CreateTestHeadlessHostInGroup(t *testing.T, queries *db.Queries, accountID, name string, status entity.HeadlessHostStatus, groupID string) db.Host {
	t.Helper()

	ensureNormalGroupExists(t, queries, groupID)

	id := uniuri.New()
	startupConfig := []byte(`{"tickRate": 60.0}`)
	connectString := "test-container-" + id

	host, err := queries.CreateHost(t.Context(), db.CreateHostParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		AccountID:                      accountID,
		CreatedBy:                      pgtype.Text{Valid: false},
		GroupID:                        groupID,
		LastStartupConfig:              startupConfig,
		LastStartupConfigSchemaVersion: 1,
		ConnectorType:                  "docker",
		ConnectString:                  connectString,
		AutoUpdatePolicy:               int32(entity.HostAutoUpdatePolicy_UNSPECIFIED),
		Memo:                           pgtype.Text{Valid: false},
		InstanceCount:                  1,
		StartedAt: pgtype.Timestamptz{
			Valid: true,
			Time:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test headless host in group %q: %v", groupID, err)
	}

	return host
}

// CreateTestHeadlessAccountInGroup は CreateTestHeadlessAccount のグループ指定版.
func CreateTestHeadlessAccountInGroup(t *testing.T, queries *db.Queries, resoniteID, credential, password, groupID string) db.HeadlessAccount {
	t.Helper()

	ensureNormalGroupExists(t, queries, groupID)

	displayName := pgtype.Text{String: "Test Headless Account", Valid: true}
	iconURL := pgtype.Text{String: "https://example.com/headless-icon.png", Valid: true}

	err := queries.CreateHeadlessAccount(t.Context(), db.CreateHeadlessAccountParams{
		ResoniteID:      resoniteID,
		Credential:      credential,
		Password:        password,
		LastDisplayName: displayName,
		LastIconUrl:     iconURL,
		GroupID:         groupID,
		CreatedBy:       pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test headless account in group %q: %v", groupID, err)
	}

	account, err := queries.GetHeadlessAccount(t.Context(), resoniteID)
	if err != nil {
		t.Fatalf("failed to get created headless account: %v", err)
	}

	return account
}

// CreateTestSessionInGroup は CreateTestSession のグループ指定版.
func CreateTestSessionInGroup(t *testing.T, queries *db.Queries, hostID, name string, status entity.SessionStatus, groupID string) db.Session {
	t.Helper()

	ensureNormalGroupExists(t, queries, groupID)

	id := uniuri.New()
	startupParams := []byte(`{"maxUsers": 8}`)

	session, err := queries.UpsertSession(t.Context(), db.UpsertSessionParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		StartedAt:                      pgtype.Timestamptz{Valid: true, Time: time.Now()},
		CreatedBy:                      pgtype.Text{Valid: false},
		GroupID:                        groupID,
		EndedAt:                        pgtype.Timestamptz{Valid: false},
		HostID:                         hostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    false,
		Memo:                           pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test session in group %q: %v", groupID, err)
	}

	return session
}

// SetupUserWithExactPermissions は normal グループに「指定 permission key だけを持つ
// カスタムロール」を作り、そのロールでユーザーを登録する. 「最小権限ちょうど」を持つ
// caller を作るためのヘルパー.
//
// userID は新規作成される. groupID が空文字なら "<userID>-test-group" を自動生成する.
// 戻り値は採用された groupID. 呼び出し側はこの groupID で対象リソースを作る.
//
// 引数 permKeys に PermKey_HostRead などの normal scope の key を渡す.
// system scope の key を含めると role の scope=normal と整合しないので、
// system 権限を含めたい場合は SetupUserWithExactSystemPermissions を併用する.
func SetupUserWithExactPermissions(t *testing.T, queries *db.Queries, userID, groupID string, permKeys []string) string {
	t.Helper()

	if groupID == "" {
		groupID = userID + "-test-group"
	}

	_ = CreateTestUser(t, queries, userID, "dummy-password")

	// normal グループを作成 (存在しない場合)
	if _, err := queries.GetGroup(t.Context(), groupID); err != nil {
		_, createErr := queries.CreateGroup(t.Context(), db.CreateGroupParams{
			ID:   groupID,
			Name: groupID,
			Type: string(entity.GroupType_Normal),
		})
		require.NoError(t, createErr, "failed to create normal group %q", groupID)
	}

	// カスタムロール (group 内 / scope=normal) を作って permKeys を付与
	roleID := "role-" + userID + "-" + uniuri.New()

	_, err := queries.CreateRole(t.Context(), db.CreateRoleParams{
		ID:      roleID,
		GroupID: pgtype.Text{String: groupID, Valid: true},
		Name:    "exact-perm-role",
		Scope:   string(entity.RoleScope_Normal),
	})
	require.NoError(t, err, "failed to create custom role")

	for _, key := range permKeys {
		err := queries.AddRolePermission(t.Context(), db.AddRolePermissionParams{
			RoleID:        roleID,
			PermissionKey: key,
		})
		require.NoError(t, err, "failed to add permission %q to role", key)
	}

	// ユーザーをそのロールでグループに追加
	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
		RoleID:  roleID,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add user to group")

	return groupID
}

// SetupUserWithExactSystemPermissions は system グループに「指定 system permission key
// だけを持つカスタムロール」(scope=system) を作り、ユーザーを登録する.
//
// "system 権限ちょうど" を持つ caller を用意するためのヘルパー.
// このユーザーは normal グループには所属しないので、normal scope の権限テストには
// 使えない. normal + system を同時に持たせる場合は両方を呼び出すこと.
//
// system scope のカスタムロールは現状未対応なので、ここではグループ単位ではなく
// global カスタムロール (group_id=NULL) として作成して system グループに割り当てる
// — と思いきや、role の scope と group の type が合致しないとアプリ層でエラーになる.
// system グループにアサイン可能な scope=system のロールを作る. group_id は system.
func SetupUserWithExactSystemPermissions(t *testing.T, queries *db.Queries, userID string, sysPermKeys []string) {
	t.Helper()

	_ = CreateTestUser(t, queries, userID, "dummy-password")

	// system グループ内のカスタムロール (scope=system).
	// system グループ singleton は migration で作られているので存在する想定.
	roleID := "role-sys-" + userID + "-" + uniuri.New()

	_, err := queries.CreateRole(t.Context(), db.CreateRoleParams{
		ID:      roleID,
		GroupID: pgtype.Text{String: entity.SystemGroupID, Valid: true},
		Name:    "exact-sys-perm-role",
		Scope:   string(entity.RoleScope_System),
	})
	require.NoError(t, err, "failed to create custom system role")

	for _, key := range sysPermKeys {
		err := queries.AddRolePermission(t.Context(), db.AddRolePermissionParams{
			RoleID:        roleID,
			PermissionKey: key,
		})
		require.NoError(t, err, "failed to add system permission %q to role", key)
	}

	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: entity.SystemGroupID,
		UserID:  userID,
		RoleID:  roleID,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add user to system group")

	// 念のため personal group も投入 (ResolveGroupIDForUser の経路で参照される).
	personalGroupID := userID + "-personal"

	_, err = queries.CreateGroup(t.Context(), db.CreateGroupParams{
		ID:   personalGroupID,
		Name: personalGroupID,
		Type: string(entity.GroupType_Personal),
	})
	require.NoError(t, err, "failed to create personal group")

	_, err = queries.AddGroupMember(t.Context(), db.AddGroupMemberParams{
		GroupID: personalGroupID,
		UserID:  userID,
		RoleID:  entity.SeedRoleID_Admin,
		AddedBy: pgtype.Text{Valid: false},
	})
	require.NoError(t, err, "failed to add user to personal group")
}

// InsertTestContainerLog inserts a test container log entry into the database.
func InsertTestContainerLog(t *testing.T, queries *db.Queries, hostID string, instanceID int32, ts time.Time, stream, logMsg string) {
	t.Helper()

	tag := fmt.Sprintf("headless-%s-%d", hostID, instanceID)
	data, err := json.Marshal(map[string]string{
		"log":    logMsg,
		"stream": stream,
	})
	require.NoError(t, err, "failed to marshal test log data")

	err = queries.InsertContainerLog(context.Background(), db.InsertContainerLogParams{
		Tag:  pgtype.Text{String: tag, Valid: true},
		Ts:   pgtype.Timestamp{Time: ts, Valid: true},
		Data: data,
	})
	require.NoError(t, err)
}
