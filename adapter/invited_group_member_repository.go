package adapter

import (
	"context"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ port.InvitedGroupMemberRepository = (*InvitedGroupMemberRepository)(nil)

type InvitedGroupMemberRepository struct {
	q *db.Queries
}

func NewInvitedGroupMemberRepository(q *db.Queries) *InvitedGroupMemberRepository {
	return &InvitedGroupMemberRepository{q: q}
}

func (r *InvitedGroupMemberRepository) Add(ctx context.Context, groupID, invitationID, roleID string, addedBy *string) (*entity.InvitedGroupMember, error) {
	rows, err := r.q.AddInvitationGroupMember(ctx, db.AddInvitationGroupMemberParams{
		GroupID:      groupID,
		RoleID:       roleID,
		AddedBy:      textFromPtr(addedBy),
		InvitationID: invitationID,
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "add invited member", 0)
	}

	if rows == 0 {
		return nil, errors.WrapPrefix(domain.ErrNotFound, "pending invitation", 0)
	}

	return r.Get(ctx, groupID, invitationID)
}

func (r *InvitedGroupMemberRepository) Remove(ctx context.Context, groupID, invitationID string) error {
	rows, err := r.q.RemoveInvitationGroupMember(ctx, db.RemoveInvitationGroupMemberParams{
		GroupID:      groupID,
		InvitationID: invitationID,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if rows == 0 {
		return errors.WrapPrefix(domain.ErrNotFound, "invited group member", 0)
	}

	return nil
}

func (r *InvitedGroupMemberRepository) UpdateRole(ctx context.Context, groupID, invitationID, roleID string) error {
	rows, err := r.q.UpdateInvitationGroupMemberRole(ctx, db.UpdateInvitationGroupMemberRoleParams{
		GroupID:      groupID,
		InvitationID: invitationID,
		RoleID:       roleID,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}

	if rows == 0 {
		return errors.WrapPrefix(domain.ErrNotFound, "invited group member", 0)
	}

	return nil
}

func (r *InvitedGroupMemberRepository) Get(ctx context.Context, groupID, invitationID string) (*entity.InvitedGroupMember, error) {
	row, err := r.q.GetInvitationGroupMember(ctx, db.GetInvitationGroupMemberParams{
		GroupID:      groupID,
		InvitationID: invitationID,
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "invited group member", 0)
	}

	return dbInvitedGroupMemberToEntity(row.InvitationGroupMember, row.ResoniteID, row.ExpiresAt.Time), nil
}

func (r *InvitedGroupMemberRepository) ListByGroup(ctx context.Context, groupID string) (entity.InvitedGroupMemberList, error) {
	rows, err := r.q.ListInvitationGroupMembersByGroup(ctx, groupID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.InvitedGroupMemberList, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbInvitedGroupMemberToEntity(row.InvitationGroupMember, row.ResoniteID, row.ExpiresAt.Time))
	}

	return result, nil
}

func dbInvitedGroupMemberToEntity(m db.InvitationGroupMember, resoniteID string, expiresAt time.Time) *entity.InvitedGroupMember {
	e := &entity.InvitedGroupMember{
		GroupID:      m.GroupID,
		InvitationID: m.InvitationID,
		ResoniteID:   resoniteID,
		RoleID:       m.RoleID,
		AddedBy:      ptrFromText(m.AddedBy),
		ExpiresAt:    expiresAt,
	}

	if m.AddedAt.Valid {
		e.AddedAt = m.AddedAt.Time
	}

	return e
}
