package usecase

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// PermissionUsecase は permission 判定の中央ロジック.
// interceptor / RPC handler から呼ばれる.
type PermissionUsecase struct {
	groupRepo  port.GroupRepository
	memberRepo port.GroupMemberRepository
	roleRepo   port.RoleRepository
}

func NewPermissionUsecase(groupRepo port.GroupRepository, memberRepo port.GroupMemberRepository, roleRepo port.RoleRepository) *PermissionUsecase {
	return &PermissionUsecase{
		groupRepo:  groupRepo,
		memberRepo: memberRepo,
		roleRepo:   roleRepo,
	}
}

// HasPermission は userID がgroupID に対して requiredKey の権限を持つか判定する.
//
// 判定ルール:
//   - requiredKey が system:* の場合: system グループ経由でその key を持つか
//   - それ以外 (normal scope) の場合:
//   - 1. userID が groupID で requiredKey を持つ
//   - 2. または、userID が system:group.manage を持つ (normal scope の代行権限)
func (u *PermissionUsecase) HasPermission(ctx context.Context, userID, groupID, requiredKey string) (bool, error) {
	if requiredKey == "" {
		return true, nil
	}

	if strings.HasPrefix(requiredKey, "system:") {
		sysPerms, err := u.memberRepo.ListUserSystemPermissions(ctx, userID)
		if err != nil {
			return false, errors.Wrap(err, 0)
		}

		return slices.Contains(sysPerms, requiredKey), nil
	}

	if groupID != "" {
		perms, err := u.memberRepo.GetUserPermissionsForGroup(ctx, userID, groupID)
		if err != nil {
			return false, errors.Wrap(err, 0)
		}

		if slices.Contains(perms, requiredKey) {
			return true, nil
		}
	}

	// system:group.manage は全 normal-scope 操作を代行できる.
	sysPerms, err := u.memberRepo.ListUserSystemPermissions(ctx, userID)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	return slices.Contains(sysPerms, entity.PermKey_SystemGroupManage), nil
}

// HasAllPermissions は requiredKeys 全てを満たすか判定する.
func (u *PermissionUsecase) HasAllPermissions(ctx context.Context, userID, groupID string, requiredKeys []string) (bool, error) {
	for _, k := range requiredKeys {
		ok, err := u.HasPermission(ctx, userID, groupID, k)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// HasSystemPermission は system 権限のみを判定する (groupID 引かない).
func (u *PermissionUsecase) HasSystemPermission(ctx context.Context, userID, key string) (bool, error) {
	sysPerms, err := u.memberRepo.ListUserSystemPermissions(ctx, userID)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	return slices.Contains(sysPerms, key), nil
}

// ResolveGroupIDForUser は user の指定 groupID 解決を行う.
// groupID が空文字なら personal group を返す.
func (u *PermissionUsecase) ResolveGroupIDForUser(ctx context.Context, userID, requestedGroupID string) (string, error) {
	if requestedGroupID != "" {
		return requestedGroupID, nil
	}

	g, err := u.groupRepo.GetPersonalGroupByUser(ctx, userID)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	return g.ID, nil
}

// ResolveListGroupFilter は List 系 RPC のグループ自動絞り込みロジック.
//
// 戻り値の groupIDs は repository / sqlc layer にそのまま渡せる:
//   - listAll == true: groupIDs は nil. 呼び出し側は全件取得すべき (system:group.list 保持者).
//   - listAll == false: groupIDs は user が permKey を持つグループ群. 空 slice の場合は
//     「ヒット 0 件」を意味する (= 所属グループが無い / read 権限を持つグループが無い).
//
// permKey が空の場合は所属グループ全件を返す (= 任意の所属でフィルタ).
func (u *PermissionUsecase) ResolveListGroupFilter(ctx context.Context, userID, permKey string) ([]string, bool, error) {
	canListAll, err := u.HasSystemPermission(ctx, userID, entity.PermKey_SystemGroupList)
	if err != nil {
		return nil, false, errors.Wrap(err, 0)
	}

	if canListAll {
		return nil, true, nil
	}

	// system:group.manage 保持者も実質全 normal-scope を代行できるが、List 系での
	// 「全件返却」は system:group.list の意味論なので別扱い.
	memberships, err := u.memberRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, false, errors.Wrap(err, 0)
	}

	result := make([]string, 0, len(memberships))

	for _, m := range memberships {
		if m.GroupID == entity.SystemGroupID {
			continue // system グループは normal-scope リソースを持たないので除外
		}

		if permKey == "" {
			result = append(result, m.GroupID)
			continue
		}

		ok, err := u.HasPermission(ctx, userID, m.GroupID, permKey)
		if err != nil {
			return nil, false, errors.Wrap(err, 0)
		}

		if ok {
			result = append(result, m.GroupID)
		}
	}

	return result, false, nil
}

// CanReadGroup は userID が groupID に対し permKey を持つか / system:group.list で
// 閲覧可能かを判定する. List 系 RPC の explicit group_id 指定時の認可に使う.
func (u *PermissionUsecase) CanReadGroup(ctx context.Context, userID, groupID, permKey string) (bool, error) {
	canList, err := u.HasSystemPermission(ctx, userID, entity.PermKey_SystemGroupList)
	if err != nil {
		return false, errors.Wrap(err, 0)
	}

	if canList {
		return true, nil
	}

	return u.HasPermission(ctx, userID, groupID, permKey)
}

// GroupPermissionSummary は GetMyPermissionsSummary が返す要素.
// user の各グループに対する有効 permission を集約してフロントエンドの
// UI 出し分けに使う.
type GroupPermissionSummary struct {
	GroupID        string
	RoleID         string
	PermissionKeys []string
}

type MyPermissionsSummary struct {
	Groups               []GroupPermissionSummary
	SystemPermissionKeys []string
}

func (u *PermissionUsecase) GetMyPermissionsSummary(ctx context.Context, userID string) (*MyPermissionsSummary, error) {
	memberships, err := u.memberRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := &MyPermissionsSummary{
		Groups:               make([]GroupPermissionSummary, 0, len(memberships)),
		SystemPermissionKeys: []string{},
	}

	for _, m := range memberships {
		perms, err := u.memberRepo.GetUserPermissionsForGroup(ctx, userID, m.GroupID)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}

		if perms == nil {
			perms = []string{}
		}

		result.Groups = append(result.Groups, GroupPermissionSummary{
			GroupID:        m.GroupID,
			RoleID:         m.RoleID,
			PermissionKeys: perms,
		})

		if m.GroupID == entity.SystemGroupID {
			result.SystemPermissionKeys = append(result.SystemPermissionKeys, perms...)
		}
	}

	return result, nil
}

// CurrentUserID は ctx から caller user_id を取り出す.
// claims が無ければ domain.ErrUnauthenticated を返す.
//
// CLI / worker など RPC 経由しない呼び出しでも、エントリポイントで
// `auth.WithActAsUser(ctx, userID)` を呼んで claims をセットしてから
// usecase に到達することを前提とする (system 操作なら "system" を渡す).
func CurrentUserID(ctx context.Context) (string, error) {
	claims, err := auth.GetAuthClaimsFromContext(ctx)
	if err != nil {
		return "", errors.Wrap(domain.ErrUnauthenticated, 0)
	}

	if claims.UserID == "" {
		return "", errors.Wrap(domain.ErrUnauthenticated, 0)
	}

	return claims.UserID, nil
}

// RequirePermissionForGroup は ctx 上の caller が groupID に対し permKey を
// 持つことを要求する. 持たなければ domain.ErrPermissionDenied を返す.
//
// usecase 層の mutating メソッドの先頭で呼び、RPC interceptor とは独立に
// リソース実体に対する権限を最終ガードする.
func (u *PermissionUsecase) RequirePermissionForGroup(ctx context.Context, groupID, permKey string) error {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return err
	}

	ok, err := u.HasPermission(ctx, userID, groupID, permKey)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if !ok {
		return errors.Wrap(fmt.Errorf("%w: %s on group %s", domain.ErrPermissionDenied, permKey, groupID), 0)
	}

	return nil
}

// RequireSystemPermission は ctx 上の caller が permKey (system:*) を持つことを要求する.
func (u *PermissionUsecase) RequireSystemPermission(ctx context.Context, permKey string) error {
	userID, err := CurrentUserID(ctx)
	if err != nil {
		return err
	}

	ok, err := u.HasSystemPermission(ctx, userID, permKey)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if !ok {
		return errors.Wrap(fmt.Errorf("%w: %s", domain.ErrPermissionDenied, permKey), 0)
	}

	return nil
}

// RequireAllPermissionsForGroup は permKeys 全てを要求する (AND).
func (u *PermissionUsecase) RequireAllPermissionsForGroup(ctx context.Context, groupID string, permKeys []string) error {
	for _, k := range permKeys {
		if err := u.RequirePermissionForGroup(ctx, groupID, k); err != nil {
			return err
		}
	}

	return nil
}

