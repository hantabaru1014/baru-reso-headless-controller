package usecase

import (
	"context"

	"github.com/dchest/uniuri"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// ErrGroupOperationForbidden は personal / system グループに対する禁止操作.
var ErrGroupOperationForbidden = errors.New("operation not allowed on this group type")

// GroupUsecase はグループの CRUD + メンバー管理を提供する.
type GroupUsecase struct {
	groupRepo  port.GroupRepository
	memberRepo port.GroupMemberRepository
	roleRepo   port.RoleRepository
	permUC     *PermissionUsecase
}

func NewGroupUsecase(
	groupRepo port.GroupRepository,
	memberRepo port.GroupMemberRepository,
	roleRepo port.RoleRepository,
	permUC *PermissionUsecase,
) *GroupUsecase {
	return &GroupUsecase{
		groupRepo:  groupRepo,
		memberRepo: memberRepo,
		roleRepo:   roleRepo,
		permUC:     permUC,
	}
}

// CreateGroup は normal グループを新規作成する. 作成者は自動的に admin として登録される.
// 権限要件: system:group.manage (RPC interceptor と独立に usecase 層でも最終ガードする).
func (u *GroupUsecase) CreateGroup(ctx context.Context, name string, creatorUserID string) (*entity.Group, error) {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemGroupManage); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, errors.New("group name is required")
	}

	id := uniuri.New()

	g, err := u.groupRepo.Create(ctx, id, name, entity.GroupType_Normal)
	if err != nil {
		return nil, err
	}

	if _, err := u.memberRepo.Add(ctx, g.ID, creatorUserID, entity.SeedRoleID_Admin, &creatorUserID); err != nil {
		return nil, errors.WrapPrefix(err, "add creator as admin", 0)
	}

	return g, nil
}

func (u *GroupUsecase) GetGroup(ctx context.Context, id string) (*entity.Group, error) {
	return u.groupRepo.Get(ctx, id)
}

// ListGroupsForUser は user の閲覧可能なグループ一覧を返す.
// system:group.list を持つ場合は全グループ、それ以外は所属するグループのみ.
func (u *GroupUsecase) ListGroupsForUser(ctx context.Context, userID string) (entity.GroupList, error) {
	canListAll, err := u.permUC.HasSystemPermission(ctx, userID, entity.PermKey_SystemGroupList)
	if err != nil {
		return nil, err
	}

	if canListAll {
		return u.groupRepo.ListAll(ctx)
	}

	return u.groupRepo.ListByUser(ctx, userID)
}

// UpdateGroupName は normal グループのみ name 変更可能. personal/system は禁止.
func (u *GroupUsecase) UpdateGroupName(ctx context.Context, id, name string) (*entity.Group, error) {
	g, err := u.groupRepo.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if g.Type != entity.GroupType_Normal {
		return nil, errors.WrapPrefix(ErrGroupOperationForbidden, "cannot rename non-normal group", 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, id, entity.PermKey_GroupEdit); err != nil {
		return nil, err
	}

	if err := u.groupRepo.UpdateName(ctx, id, name); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return u.groupRepo.Get(ctx, id)
}

// DeleteGroup は normal グループのみ削除可能. personal/system は禁止.
func (u *GroupUsecase) DeleteGroup(ctx context.Context, id string) error {
	g, err := u.groupRepo.Get(ctx, id)
	if err != nil {
		return err
	}

	if g.Type != entity.GroupType_Normal {
		return errors.WrapPrefix(ErrGroupOperationForbidden, "cannot delete non-normal group", 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, id, entity.PermKey_GroupEdit); err != nil {
		return err
	}

	return u.groupRepo.Delete(ctx, id)
}

func (u *GroupUsecase) ListGroupMembers(ctx context.Context, groupID string) (entity.GroupMemberList, error) {
	return u.memberRepo.ListByGroup(ctx, groupID)
}

// AddGroupMember は group に user を role で登録する. personal グループへの追加は禁止.
func (u *GroupUsecase) AddGroupMember(ctx context.Context, groupID, userID, roleID string, addedBy *string) (*entity.GroupMember, error) {
	// permission チェックを存在チェックより先に行う. 権限の無い caller に
	// NotFound / PermissionDenied の違いでグループ存在 (ひいては
	// `<user-id>-personal` 形式からユーザー存在) を漏らさないため.
	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_GroupMembersManage); err != nil {
		return nil, err
	}

	g, err := u.groupRepo.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}

	if g.Type == entity.GroupType_Personal {
		return nil, errors.WrapPrefix(ErrGroupOperationForbidden, "cannot add member to personal group", 0)
	}

	role, err := u.roleRepo.Get(ctx, roleID)
	if err != nil {
		return nil, err
	}

	if !isRoleAssignableToGroup(role, g) {
		return nil, errors.New("role scope does not match group type")
	}

	// privilege escalation 防止: caller が持っていない権限を含むロールでの登録を拒否.
	// system グループへの登録 (system-scope ロール) は HasPermission の system bypass で
	// 透過的にスキップされる.
	if err := u.requirePermSubsetOfCaller(ctx, g, role); err != nil {
		return nil, err
	}

	return u.memberRepo.Add(ctx, groupID, userID, roleID, addedBy)
}

// RemoveGroupMember は group から user を外す. personal グループからの削除は禁止.
// system グループの最後のメンバーは削除を拒否する (システムが復旧不能になるため).
func (u *GroupUsecase) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	g, err := u.groupRepo.Get(ctx, groupID)
	if err != nil {
		return err
	}

	if g.Type == entity.GroupType_Personal {
		return errors.WrapPrefix(ErrGroupOperationForbidden, "cannot remove member from personal group", 0)
	}

	if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_GroupMembersManage); err != nil {
		return err
	}

	// system グループから最後のメンバーを抜くと system:* 権限保有者がゼロになり、
	// brhcli system-admin add 以外で復旧不能になるため拒否する.
	if g.Type == entity.GroupType_System {
		members, err := u.memberRepo.ListByGroup(ctx, groupID)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		hasOther := false

		for _, m := range members {
			if m.UserID != userID {
				hasOther = true
				break
			}
		}

		if !hasOther {
			return errors.WrapPrefix(ErrGroupOperationForbidden, "cannot remove the last member of the system group", 0)
		}
	}

	return u.memberRepo.Remove(ctx, groupID, userID)
}

// UpdateGroupMemberRole は member のロールを変更する.
// 仕様: 11. セキュリティ上の不変条件 — personal の本人ロールは system:group.manage 経由でのみ変更可.
func (u *GroupUsecase) UpdateGroupMemberRole(ctx context.Context, groupID, userID, roleID string) (*entity.GroupMember, error) {
	g, err := u.groupRepo.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}

	// permission 要件: personal は system:group.manage, それ以外は group:members.manage.
	if g.Type == entity.GroupType_Personal {
		if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemGroupManage); err != nil {
			return nil, err
		}
	} else {
		if err := u.permUC.RequirePermissionForGroup(ctx, groupID, entity.PermKey_GroupMembersManage); err != nil {
			return nil, err
		}
	}

	role, err := u.roleRepo.Get(ctx, roleID)
	if err != nil {
		return nil, err
	}

	if !isRoleAssignableToGroup(role, g) {
		return nil, errors.New("role scope does not match group type")
	}

	// privilege escalation 防止: caller が持っていない権限を含むロールへの変更を拒否.
	if err := u.requirePermSubsetOfCaller(ctx, g, role); err != nil {
		return nil, err
	}

	if _, err := u.memberRepo.Get(ctx, groupID, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}

		return nil, errors.Wrap(err, 0)
	}

	if err := u.memberRepo.UpdateRole(ctx, groupID, userID, roleID); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return u.memberRepo.Get(ctx, groupID, userID)
}

// requirePermSubsetOfCaller は role が持つ permission を全て caller が group 上で
// 持っていることを要求する. role の permission を介した privilege escalation を防ぐ.
//
// system-scope role / system グループへの assignment は HasPermission の
// system:group.manage bypass や system permission 経由で透過的に成立する.
//nolint:funcorder // 関連 method (AddGroupMember / UpdateGroupMemberRole) 直下にヘルパーを置く方が読みやすい
func (u *GroupUsecase) requirePermSubsetOfCaller(ctx context.Context, group *entity.Group, role *entity.Role) error {
	if len(role.PermissionKeys) == 0 {
		return nil
	}

	callerID, err := CurrentUserID(ctx)
	if err != nil {
		return err
	}

	for _, k := range role.PermissionKeys {
		ok, err := u.permUC.HasPermission(ctx, callerID, group.ID, k)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if !ok {
			return errors.WrapPrefix(domain.ErrPermissionDenied, "cannot assign role granting permission caller does not hold: "+k, 0)
		}
	}

	return nil
}

func isRoleAssignableToGroup(role *entity.Role, group *entity.Group) bool {
	switch role.Scope {
	case entity.RoleScope_Normal:
		return group.Type == entity.GroupType_Normal || group.Type == entity.GroupType_Personal
	case entity.RoleScope_System:
		return group.Type == entity.GroupType_System
	}

	return false
}

// ValidatePersonalRoleAssignable は personal グループのメンバーシップに付与する
// ロールとして roleID が妥当か検証する. 招待トークン発行 / ユーザー直接作成の
// 入口で呼び、不正な role id・scope 違反・他グループ専用ロールの混入を防ぐ.
func (u *GroupUsecase) ValidatePersonalRoleAssignable(ctx context.Context, roleID string) error {
	role, err := u.roleRepo.Get(ctx, roleID)
	if err != nil {
		return err
	}

	if role.Scope != entity.RoleScope_Normal {
		return errors.New("personal group role must be a normal-scope role")
	}

	if role.GroupID != nil && *role.GroupID != "" {
		return errors.New("personal group role must be a global role")
	}

	return nil
}

// EnsurePersonalGroupForUser は user の personal グループを取得 / 作成する.
// 引数の roleID は personal メンバーシップに付与するロール (デフォルト seed-admin).
func (u *GroupUsecase) EnsurePersonalGroupForUser(ctx context.Context, userID, roleID string) (*entity.Group, error) {
	if roleID != "" {
		if err := u.ValidatePersonalRoleAssignable(ctx, roleID); err != nil {
			return nil, err
		}
	}

	if g, err := u.groupRepo.GetPersonalGroupByUser(ctx, userID); err == nil {
		return g, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, errors.Wrap(err, 0)
	}

	gid := userID + "-personal"

	g, err := u.groupRepo.Create(ctx, gid, gid, entity.GroupType_Personal)
	if err != nil {
		return nil, errors.WrapPrefix(err, "create personal group", 0)
	}

	if roleID == "" {
		roleID = entity.SeedRoleID_Admin
	}

	if _, err := u.memberRepo.Add(ctx, g.ID, userID, roleID, nil); err != nil {
		return nil, errors.WrapPrefix(err, "register personal member", 0)
	}

	return g, nil
}
