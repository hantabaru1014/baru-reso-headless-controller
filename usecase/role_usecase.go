package usecase

import (
	"context"
	"strings"

	"github.com/dchest/uniuri"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var (
	// ErrCannotModifyBuiltinRole は seed ロールへの編集/削除試行.
	ErrCannotModifyBuiltinRole = errors.New("cannot modify builtin role")
	// ErrInvalidPermissionKey は AllPermissionKeys に含まれない key を指定された.
	ErrInvalidPermissionKey = errors.New("invalid permission_key")
)

// RoleUsecase はカスタムロール CRUD + permission 一覧を提供する.
type RoleUsecase struct {
	roleRepo  port.RoleRepository
	groupRepo port.GroupRepository
	permUC    *PermissionUsecase
}

func NewRoleUsecase(roleRepo port.RoleRepository, groupRepo port.GroupRepository, permUC *PermissionUsecase) *RoleUsecase {
	return &RoleUsecase{
		roleRepo:  roleRepo,
		groupRepo: groupRepo,
		permUC:    permUC,
	}
}

// ListRoles は ListRoles RPC の戻り値.
// group_id 未指定: グローバルロール (seed + グローバルカスタム).
// 指定時: そのグループに割り当て可能なロール (scope 一致のグローバル + グループ内カスタム).
func (u *RoleUsecase) ListRoles(ctx context.Context, groupID *string) (entity.RoleList, error) {
	if groupID == nil || *groupID == "" {
		return u.roleRepo.ListGlobal(ctx)
	}

	g, err := u.groupRepo.Get(ctx, *groupID)
	if err != nil {
		return nil, err
	}

	scope := entity.RoleScope_Normal
	if g.Type == entity.GroupType_System {
		scope = entity.RoleScope_System
	}

	return u.roleRepo.ListAssignable(ctx, groupID, scope)
}

type CreateRoleParams struct {
	GroupID        *string
	Name           string
	Scope          entity.RoleScope
	PermissionKeys []string
}

// CreateRole は新しいカスタムロールを作成する.
func (u *RoleUsecase) CreateRole(ctx context.Context, params CreateRoleParams) (*entity.Role, error) {
	// ===== 入力検証 (書き込み前に全部済ませる) =====
	if params.Name == "" {
		return nil, errors.New("role name is required")
	}

	if params.Scope != entity.RoleScope_Normal && params.Scope != entity.RoleScope_System {
		return nil, errors.New("invalid scope")
	}

	if err := validatePermissionKeys(params.Scope, params.PermissionKeys); err != nil {
		return nil, err
	}

	isGroupScoped := params.GroupID != nil && *params.GroupID != ""

	if isGroupScoped {
		g, err := u.groupRepo.Get(ctx, *params.GroupID)
		if err != nil {
			return nil, err
		}

		if !canScopeBeAttachedToGroup(params.Scope, g.Type) {
			return nil, errors.New("scope does not match group type")
		}
	}

	// ===== 認可 (caller の権限) =====
	// 権限要件: group_id 指定なら group:members.manage on group, 未指定 (グローバル) なら system:role.manage.
	if isGroupScoped {
		if err := u.permUC.RequirePermissionForGroup(ctx, *params.GroupID, entity.PermKey_GroupMembersManage); err != nil {
			return nil, err
		}

		// privilege escalation 防止: caller が持っていない権限を含むロールを作成不能にする.
		// グローバルロール側は system:role.manage 保持者 (= system-admin) のみが触れるため
		// 別途の subset check は行わない.
		if err := u.requireSubsetOfCaller(ctx, *params.GroupID, params.PermissionKeys); err != nil {
			return nil, err
		}
	} else {
		if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemRoleManage); err != nil {
			return nil, err
		}
	}

	// ===== 書き込み =====
	id := "role-" + uniuri.New()

	role, err := u.roleRepo.Create(ctx, id, params.GroupID, params.Name, params.Scope)
	if err != nil {
		return nil, err
	}

	if len(params.PermissionKeys) > 0 {
		if err := u.roleRepo.ReplacePermissions(ctx, role.ID, params.PermissionKeys); err != nil {
			return nil, err
		}

		role.PermissionKeys = params.PermissionKeys
	}

	return role, nil
}

type UpdateRoleParams struct {
	ID                string
	Name              *string
	PermissionKeys    *[]string // nil なら変更なし, 空配列なら全削除
	OverwritePermKeys bool      // true なら PermissionKeys を反映 (nil でも空配列扱い)
}

func (u *RoleUsecase) UpdateRole(ctx context.Context, params UpdateRoleParams) (*entity.Role, error) {
	role, err := u.roleRepo.Get(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	if role.IsBuiltin {
		return nil, errors.Wrap(ErrCannotModifyBuiltinRole, 0)
	}

	// ===== 入力検証 (書き込み前に全部済ませる) =====
	var newKeys []string

	if params.OverwritePermKeys {
		newKeys = []string{}
		if params.PermissionKeys != nil {
			newKeys = *params.PermissionKeys
		}

		if err := validatePermissionKeys(role.Scope, newKeys); err != nil {
			return nil, err
		}
	}

	// ===== 認可 =====
	if err := u.requireRoleMutation(ctx, role); err != nil {
		return nil, err
	}

	// privilege escalation 防止: group-scoped role の permission を変更する場合、
	// caller が持っていない権限を新たに含めることはできない.
	if params.OverwritePermKeys && role.GroupID != nil && *role.GroupID != "" {
		if err := u.requireSubsetOfCaller(ctx, *role.GroupID, newKeys); err != nil {
			return nil, err
		}
	}

	// ===== 書き込み (validation 後にまとめて実行) =====
	if params.Name != nil && *params.Name != role.Name {
		if err := u.roleRepo.UpdateName(ctx, params.ID, *params.Name); err != nil {
			return nil, errors.Wrap(err, 0)
		}
	}

	if params.OverwritePermKeys {
		if err := u.roleRepo.ReplacePermissions(ctx, params.ID, newKeys); err != nil {
			return nil, err
		}
	}

	return u.roleRepo.Get(ctx, params.ID)
}

func (u *RoleUsecase) DeleteRole(ctx context.Context, id string) error {
	role, err := u.roleRepo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}

		return errors.Wrap(err, 0)
	}

	if role.IsBuiltin {
		return errors.Wrap(ErrCannotModifyBuiltinRole, 0)
	}

	if err := u.requireRoleMutation(ctx, role); err != nil {
		return err
	}

	return u.roleRepo.Delete(ctx, id)
}

// requireRoleMutation は role の所属 (global / グループ内) に応じた permission を要求する.
// グローバル (role.GroupID == nil): system:role.manage.
// グループ内: group:members.manage on role.GroupID.
func (u *RoleUsecase) requireRoleMutation(ctx context.Context, role *entity.Role) error {
	if role.GroupID == nil || *role.GroupID == "" {
		return u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemRoleManage)
	}

	return u.permUC.RequirePermissionForGroup(ctx, *role.GroupID, entity.PermKey_GroupMembersManage)
}

func (u *RoleUsecase) GetRole(ctx context.Context, id string) (*entity.Role, error) {
	return u.roleRepo.Get(ctx, id)
}

// validatePermissionKeys は key の存在と role.scope との整合性をチェックする.
// normal scope のロールは system:* を含められず、system scope のロールは
// system:* 以外を含められない.
func validatePermissionKeys(scope entity.RoleScope, keys []string) error {
	for _, k := range keys {
		if !entity.IsValidPermissionKey(k) {
			return errors.WrapPrefix(ErrInvalidPermissionKey, k, 0)
		}

		isSystemKey := strings.HasPrefix(k, "system:")

		switch scope {
		case entity.RoleScope_Normal:
			if isSystemKey {
				return errors.WrapPrefix(ErrInvalidPermissionKey, "normal-scope role cannot contain system: key: "+k, 0)
			}
		case entity.RoleScope_System:
			if !isSystemKey {
				return errors.WrapPrefix(ErrInvalidPermissionKey, "system-scope role can only contain system: keys: "+k, 0)
			}
		}
	}

	return nil
}

// requireSubsetOfCaller は ctx 上の caller が groupID 上で keys を全て持っている
// ことを要求する. role の作成/編集/付与時に「caller が持っていない権限を grant
// できないようにする」(privilege subset rule). system:group.manage 経由の bypass は
// HasPermission 内で自動的に効くので system admin は素通りする.
func (u *RoleUsecase) requireSubsetOfCaller(ctx context.Context, groupID string, keys []string) error {
	callerID, err := CurrentUserID(ctx)
	if err != nil {
		return err
	}

	for _, k := range keys {
		ok, err := u.permUC.HasPermission(ctx, callerID, groupID, k)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if !ok {
			return errors.WrapPrefix(domain.ErrPermissionDenied, "cannot grant permission key the caller does not hold: "+k, 0)
		}
	}

	return nil
}

func canScopeBeAttachedToGroup(scope entity.RoleScope, gtype entity.GroupType) bool {
	switch scope {
	case entity.RoleScope_Normal:
		return gtype == entity.GroupType_Normal || gtype == entity.GroupType_Personal
	case entity.RoleScope_System:
		return gtype == entity.GroupType_System
	}

	return false
}
