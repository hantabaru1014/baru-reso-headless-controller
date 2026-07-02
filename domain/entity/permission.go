package entity

import "time"

// GroupType はグループの種別.
type GroupType string

const (
	GroupType_Personal GroupType = "personal"
	GroupType_Normal   GroupType = "normal"
	GroupType_System   GroupType = "system"
)

// RoleScope はロールが割り当て可能なグループ種別の範囲.
type RoleScope string

const (
	RoleScope_Normal RoleScope = "normal"
	RoleScope_System RoleScope = "system"
)

// MigratedPrePermissionGroupID は権限システム導入前の既存データの所属先となる
// "移行前全体グループ" の ID. マイグレーションで自動投入される.
const MigratedPrePermissionGroupID = "migrated-pre-permission"

// SystemGroupID は singleton な system グループの ID.
const SystemGroupID = "system"

// Seed ロール ID.
const (
	SeedRoleID_Admin           = "seed-admin"
	SeedRoleID_User            = "seed-user"
	SeedRoleID_SessionOperator = "seed-session-operator"
	SeedRoleID_SystemAdmin     = "seed-system-admin"
)

// 既知の permission key.
const (
	PermKey_HostRead             = "host:read"
	PermKey_HostWrite            = "host:write"
	PermKey_HostUse              = "host:use"
	PermKey_SessionRead          = "session:read"
	PermKey_SessionWrite         = "session:write"
	PermKey_AccountRead          = "account:read"
	PermKey_AccountWrite         = "account:write"
	PermKey_AccountUse           = "account:use"
	PermKey_GroupMembersManage   = "group:members.manage"
	PermKey_GroupEdit            = "group:edit"
	PermKey_SystemUserCreate     = "system:user.create"
	PermKey_SystemUserDelete     = "system:user.delete"
	PermKey_SystemUserList       = "system:user.list"
	PermKey_SystemGroupList      = "system:group.list"
	PermKey_SystemGroupManage    = "system:group.manage"
	PermKey_SystemRoleManage     = "system:role.manage"
)

// Group は権限スコープ単位のグループ.
type Group struct {
	ID        string
	Name      string
	Type      GroupType
	CreatedAt time.Time
	UpdatedAt time.Time
}

type GroupList []*Group

// GroupMember はグループへのユーザー所属.
type GroupMember struct {
	GroupID  string
	UserID   string
	RoleID   string
	AddedBy  *string
	JoinedAt time.Time
}

type GroupMemberList []*GroupMember

// Role はパーミッション集合.
type Role struct {
	ID             string
	GroupID        *string // NULL: グローバル
	Name           string
	Scope          RoleScope
	IsBuiltin      bool
	PermissionKeys []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RoleList []*Role

// PermissionKeyDef はフロント向け permission_key 説明.
type PermissionKeyDef struct {
	Key         string
	Description string
	Scope       RoleScope
}

// AllPermissionKeys は ListPermissions RPC で返す利用可能 key 一覧.
// 仕様書 docs/permissions.md の 4 章と完全一致させる.
var AllPermissionKeys = []PermissionKeyDef{
	{Key: PermKey_HostRead, Description: "View host list / detail / logs", Scope: RoleScope_Normal},
	{Key: PermKey_HostWrite, Description: "Create / update / start / stop / restart / delete hosts", Scope: RoleScope_Normal},
	{Key: PermKey_HostUse, Description: "Start sessions on a host", Scope: RoleScope_Normal},
	{Key: PermKey_SessionRead, Description: "View session list / detail", Scope: RoleScope_Normal},
	{Key: PermKey_SessionWrite, Description: "Create / update / stop sessions, invite / kick / ban / role changes", Scope: RoleScope_Normal},
	{Key: PermKey_AccountRead, Description: "View account list / detail / storage info / contacts", Scope: RoleScope_Normal},
	{Key: PermKey_AccountWrite, Description: "Create / update credentials / delete accounts", Scope: RoleScope_Normal},
	{Key: PermKey_AccountUse, Description: "Use an account to start a session, read / send contact DMs", Scope: RoleScope_Normal},
	{Key: PermKey_GroupMembersManage, Description: "Manage members and group-local custom roles", Scope: RoleScope_Normal},
	{Key: PermKey_GroupEdit, Description: "Edit group metadata (name etc.)", Scope: RoleScope_Normal},
	{Key: PermKey_SystemUserCreate, Description: "Create system user accounts", Scope: RoleScope_System},
	{Key: PermKey_SystemUserDelete, Description: "Delete system user accounts", Scope: RoleScope_System},
	{Key: PermKey_SystemUserList, Description: "List all system users", Scope: RoleScope_System},
	{Key: PermKey_SystemGroupList, Description: "List all groups", Scope: RoleScope_System},
	{Key: PermKey_SystemGroupManage, Description: "Manage any group (including personal), and personal role changes", Scope: RoleScope_System},
	{Key: PermKey_SystemRoleManage, Description: "Manage global custom roles", Scope: RoleScope_System},
}

// IsValidPermissionKey は AllPermissionKeys に含まれる key か検証する.
func IsValidPermissionKey(key string) bool {
	for _, p := range AllPermissionKeys {
		if p.Key == key {
			return true
		}
	}

	return false
}

// GetPermissionKeyDef は key からその定義を返す. 未知ならnil.
func GetPermissionKeyDef(key string) *PermissionKeyDef {
	for _, p := range AllPermissionKeys {
		if p.Key == key {
			return &p
		}
	}

	return nil
}
