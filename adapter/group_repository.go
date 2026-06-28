package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.GroupRepository = (*GroupRepository)(nil)

type GroupRepository struct {
	q *db.Queries
}

func NewGroupRepository(q *db.Queries) *GroupRepository {
	return &GroupRepository{q: q}
}

func (r *GroupRepository) Create(ctx context.Context, id, name string, gtype entity.GroupType) (*entity.Group, error) {
	g, err := r.q.CreateGroup(ctx, db.CreateGroupParams{
		ID:   id,
		Name: name,
		Type: string(gtype),
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "create group", 0)
	}

	return dbGroupToEntity(g), nil
}

func (r *GroupRepository) Get(ctx context.Context, id string) (*entity.Group, error) {
	g, err := r.q.GetGroup(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "group", 0)
	}

	return dbGroupToEntity(g), nil
}

func (r *GroupRepository) ListAll(ctx context.Context) (entity.GroupList, error) {
	rows, err := r.q.ListGroups(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.GroupList, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbGroupToEntity(row))
	}

	return result, nil
}

func (r *GroupRepository) ListByUser(ctx context.Context, userID string) (entity.GroupList, error) {
	rows, err := r.q.ListGroupsByUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.GroupList, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbGroupToEntity(row.Group))
	}

	return result, nil
}

func (r *GroupRepository) GetPersonalGroupByUser(ctx context.Context, userID string) (*entity.Group, error) {
	g, err := r.q.GetPersonalGroupByUser(ctx, userID)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "personal group", 0)
	}

	return dbGroupToEntity(g), nil
}

func (r *GroupRepository) UpdateName(ctx context.Context, id, name string) error {
	return r.q.UpdateGroupName(ctx, db.UpdateGroupNameParams{
		ID:   id,
		Name: name,
	})
}

func (r *GroupRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteGroup(ctx, id)
}

func dbGroupToEntity(g db.Group) *entity.Group {
	e := &entity.Group{
		ID:   g.ID,
		Name: g.Name,
		Type: entity.GroupType(g.Type),
	}
	if g.CreatedAt.Valid {
		e.CreatedAt = g.CreatedAt.Time
	}

	if g.UpdatedAt.Valid {
		e.UpdatedAt = g.UpdatedAt.Time
	}

	return e
}

// pgtypeTextFromOptString は *string を pgtype.Text に変換するヘルパー (nil なら invalid).
//
//nolint:unused // 将来 group repository 拡張時に使う予定
func pgtypeTextFromOptString(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: *s, Valid: true}
}
