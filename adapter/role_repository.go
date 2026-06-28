package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.RoleRepository = (*RoleRepository)(nil)

type RoleRepository struct {
	q *db.Queries
}

func NewRoleRepository(q *db.Queries) *RoleRepository {
	return &RoleRepository{q: q}
}

func (r *RoleRepository) Create(ctx context.Context, id string, groupID *string, name string, scope entity.RoleScope) (*entity.Role, error) {
	gid := pgtype.Text{}
	if groupID != nil {
		gid = pgtype.Text{String: *groupID, Valid: true}
	}

	role, err := r.q.CreateRole(ctx, db.CreateRoleParams{
		ID:      id,
		GroupID: gid,
		Name:    name,
		Scope:   string(scope),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "create role", 0)
	}

	e := dbRoleToEntity(role)
	e.PermissionKeys = []string{}

	return e, nil
}

func (r *RoleRepository) Get(ctx context.Context, id string) (*entity.Role, error) {
	role, err := r.q.GetRole(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "role", 0)
	}

	e := dbRoleToEntity(role)

	keys, err := r.q.ListRolePermissions(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	e.PermissionKeys = keys
	if e.PermissionKeys == nil {
		e.PermissionKeys = []string{}
	}

	return e, nil
}

func (r *RoleRepository) ListGlobal(ctx context.Context) (entity.RoleList, error) {
	rows, err := r.q.ListGlobalRoles(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return r.attachPermissionsToRoles(ctx, rows)
}

func (r *RoleRepository) ListAssignable(ctx context.Context, groupID *string, scope entity.RoleScope) (entity.RoleList, error) {
	gid := pgtype.Text{}
	if groupID != nil {
		gid = pgtype.Text{String: *groupID, Valid: true}
	}

	rows, err := r.q.ListAssignableRoles(ctx, db.ListAssignableRolesParams{
		GroupID: gid,
		Scope:   string(scope),
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return r.attachPermissionsToRoles(ctx, rows)
}

func (r *RoleRepository) UpdateName(ctx context.Context, id, name string) error {
	return r.q.UpdateRoleName(ctx, db.UpdateRoleNameParams{
		ID:   id,
		Name: name,
	})
}

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteRole(ctx, id)
}

// ReplacePermissions は与えられた key で role の permission を完全置換する.
// 差分を計算して既存と異なる分のみ INSERT/DELETE する (DB トリガーで builtin role
// は INSERT/DELETE が阻止されるため、空配列で何もない場合は touch しない).
func (r *RoleRepository) ReplacePermissions(ctx context.Context, roleID string, keys []string) error {
	existing, err := r.q.ListRolePermissions(ctx, roleID)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, k := range existing {
		existingSet[k] = struct{}{}
	}

	wantSet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		wantSet[k] = struct{}{}
	}

	// 削除分
	for k := range existingSet {
		if _, ok := wantSet[k]; !ok {
			if err := r.q.RemoveRolePermission(ctx, db.RemoveRolePermissionParams{
				RoleID:        roleID,
				PermissionKey: k,
			}); err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	// 追加分
	for k := range wantSet {
		if _, ok := existingSet[k]; !ok {
			if err := r.q.AddRolePermission(ctx, db.AddRolePermissionParams{
				RoleID:        roleID,
				PermissionKey: k,
			}); err != nil {
				return errors.Wrap(err, 0)
			}
		}
	}

	return nil
}

func (r *RoleRepository) GetPermissions(ctx context.Context, roleID string) ([]string, error) {
	keys, err := r.q.ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if keys == nil {
		keys = []string{}
	}

	return keys, nil
}

func (r *RoleRepository) attachPermissionsToRoles(ctx context.Context, rows []db.Role) (entity.RoleList, error) {
	if len(rows) == 0 {
		return entity.RoleList{}, nil
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	perms, err := r.q.ListRolePermissionsForRoles(ctx, ids)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	keysByRole := map[string][]string{}
	for _, p := range perms {
		keysByRole[p.RoleID] = append(keysByRole[p.RoleID], p.PermissionKey)
	}

	result := make(entity.RoleList, 0, len(rows))

	for _, row := range rows {
		e := dbRoleToEntity(row)
		e.PermissionKeys = keysByRole[row.ID]

		if e.PermissionKeys == nil {
			e.PermissionKeys = []string{}
		}

		result = append(result, e)
	}

	return result, nil
}

func dbRoleToEntity(r db.Role) *entity.Role {
	e := &entity.Role{
		ID:        r.ID,
		Name:      r.Name,
		Scope:     entity.RoleScope(r.Scope),
		IsBuiltin: r.IsBuiltin,
	}
	if r.GroupID.Valid {
		s := r.GroupID.String
		e.GroupID = &s
	}

	if r.CreatedAt.Valid {
		e.CreatedAt = r.CreatedAt.Time
	}

	if r.UpdatedAt.Valid {
		e.UpdatedAt = r.UpdatedAt.Time
	}

	return e
}
