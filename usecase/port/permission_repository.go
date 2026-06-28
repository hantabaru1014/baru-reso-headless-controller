// Package port は権限システム関連の repository interface を定義する.
// 実装は adapter/{group,role,group_member}_repository.go.
package port

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

// GroupRepository はグループ (権限スコープ単位) の永続化を担う.
type GroupRepository interface {
	Create(ctx context.Context, id, name string, gtype entity.GroupType) (*entity.Group, error)
	Get(ctx context.Context, id string) (*entity.Group, error)
	ListAll(ctx context.Context) (entity.GroupList, error)
	ListByUser(ctx context.Context, userID string) (entity.GroupList, error)
	GetPersonalGroupByUser(ctx context.Context, userID string) (*entity.Group, error)
	UpdateName(ctx context.Context, id, name string) error
	Delete(ctx context.Context, id string) error
}

// RoleRepository はロール (パーミッション集合) の永続化を担う.
type RoleRepository interface {
	Create(ctx context.Context, id string, groupID *string, name string, scope entity.RoleScope) (*entity.Role, error)
	Get(ctx context.Context, id string) (*entity.Role, error)
	// グローバルロール (seed + グローバルカスタム).
	ListGlobal(ctx context.Context) (entity.RoleList, error)
	// 指定グループに割り当て可能なロール (グローバル + 当該グループ内, scope一致のみ).
	ListAssignable(ctx context.Context, groupID *string, scope entity.RoleScope) (entity.RoleList, error)
	UpdateName(ctx context.Context, id, name string) error
	Delete(ctx context.Context, id string) error
	// permission_keys を完全置換する (空配列なら全削除).
	ReplacePermissions(ctx context.Context, roleID string, keys []string) error
	GetPermissions(ctx context.Context, roleID string) ([]string, error)
}

// GroupMemberRepository は (group, user, role) の所属関係を扱う.
type GroupMemberRepository interface {
	Add(ctx context.Context, groupID, userID, roleID string, addedBy *string) (*entity.GroupMember, error)
	Remove(ctx context.Context, groupID, userID string) error
	UpdateRole(ctx context.Context, groupID, userID, roleID string) error
	Get(ctx context.Context, groupID, userID string) (*entity.GroupMember, error)
	ListByGroup(ctx context.Context, groupID string) (entity.GroupMemberList, error)
	ListByUser(ctx context.Context, userID string) (entity.GroupMemberList, error)
	// 指定 (user, group) に対する permission_key を引く. グループ非所属なら empty.
	GetUserPermissionsForGroup(ctx context.Context, userID, groupID string) ([]string, error)
	// system グループ経由の permission_key 一覧.
	ListUserSystemPermissions(ctx context.Context, userID string) ([]string, error)
}
