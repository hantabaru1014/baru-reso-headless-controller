package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.GroupMemberRepository = (*GroupMemberRepository)(nil)

type GroupMemberRepository struct {
	q *db.Queries
}

func NewGroupMemberRepository(q *db.Queries) *GroupMemberRepository {
	return &GroupMemberRepository{q: q}
}

func (r *GroupMemberRepository) Add(ctx context.Context, groupID, userID, roleID string, addedBy *string) (*entity.GroupMember, error) {
	addedByText := pgtype.Text{}
	if addedBy != nil {
		addedByText = pgtype.Text{String: *addedBy, Valid: true}
	}

	m, err := r.q.AddGroupMember(ctx, db.AddGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
		RoleID:  roleID,
		AddedBy: addedByText,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "add member", 0)
	}

	return dbGroupMemberToEntity(m), nil
}

func (r *GroupMemberRepository) Remove(ctx context.Context, groupID, userID string) error {
	return r.q.RemoveGroupMember(ctx, db.RemoveGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
	})
}

func (r *GroupMemberRepository) UpdateRole(ctx context.Context, groupID, userID, roleID string) error {
	return r.q.UpdateGroupMemberRole(ctx, db.UpdateGroupMemberRoleParams{
		GroupID: groupID,
		UserID:  userID,
		RoleID:  roleID,
	})
}

func (r *GroupMemberRepository) Get(ctx context.Context, groupID, userID string) (*entity.GroupMember, error) {
	m, err := r.q.GetGroupMember(ctx, db.GetGroupMemberParams{
		GroupID: groupID,
		UserID:  userID,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "group member", 0)
	}

	return dbGroupMemberToEntity(m), nil
}

func (r *GroupMemberRepository) ListByGroup(ctx context.Context, groupID string) (entity.GroupMemberList, error) {
	rows, err := r.q.ListGroupMembers(ctx, groupID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.GroupMemberList, 0, len(rows))
	for _, m := range rows {
		result = append(result, dbGroupMemberToEntity(m))
	}

	return result, nil
}

func (r *GroupMemberRepository) ListByUser(ctx context.Context, userID string) (entity.GroupMemberList, error) {
	rows, err := r.q.ListUserGroupMemberships(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.GroupMemberList, 0, len(rows))
	for _, m := range rows {
		result = append(result, dbGroupMemberToEntity(m))
	}

	return result, nil
}

func (r *GroupMemberRepository) GetUserPermissionsForGroup(ctx context.Context, userID, groupID string) ([]string, error) {
	keys, err := r.q.GetUserPermissionsForGroup(ctx, db.GetUserPermissionsForGroupParams{
		UserID:  userID,
		GroupID: groupID,
	})
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if keys == nil {
		keys = []string{}
	}

	return keys, nil
}

func (r *GroupMemberRepository) ListUserSystemPermissions(ctx context.Context, userID string) ([]string, error) {
	keys, err := r.q.ListUserSystemPermissions(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if keys == nil {
		keys = []string{}
	}

	return keys, nil
}

func dbGroupMemberToEntity(m db.GroupMember) *entity.GroupMember {
	e := &entity.GroupMember{
		GroupID: m.GroupID,
		UserID:  m.UserID,
		RoleID:  m.RoleID,
	}
	if m.AddedBy.Valid {
		s := m.AddedBy.String
		e.AddedBy = &s
	}

	if m.JoinedAt.Valid {
		e.JoinedAt = m.JoinedAt.Time
	}

	return e
}
