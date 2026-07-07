package adapter

import (
	"context"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ port.MessageRepository = (*MessageRepository)(nil)

type MessageRepository struct {
	q *db.Queries
}

func NewMessageRepository(q *db.Queries) *MessageRepository {
	return &MessageRepository{q: q}
}

func (r *MessageRepository) Create(ctx context.Context, id, title, body string, groupID *string, createdBy string) (*entity.Message, error) {
	msg, err := r.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:        id,
		Title:     title,
		Body:      body,
		GroupID:   textFromPtr(groupID),
		CreatedBy: pgtype.Text{String: createdBy, Valid: true},
	})
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "create message", 0)
	}

	// Create は group_name を返さないため、解決済みの entity を得るために Get し直す.
	return r.Get(ctx, msg.ID)
}

func (r *MessageRepository) Get(ctx context.Context, id string) (*entity.Message, error) {
	row, err := r.q.GetMessage(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "message", 0)
	}

	return dbMessageToEntity(row.Message, row.GroupName), nil
}

func (r *MessageRepository) ListAll(ctx context.Context) (entity.MessageList, error) {
	rows, err := r.q.ListAllMessages(ctx)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.MessageList, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbMessageToEntity(row.Message, row.GroupName))
	}

	return result, nil
}

func (r *MessageRepository) ListVisibleToUser(ctx context.Context, userID string) (entity.MessageList, error) {
	rows, err := r.q.ListMessagesVisibleToUser(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	result := make(entity.MessageList, 0, len(rows))
	for _, row := range rows {
		result = append(result, dbMessageToEntity(row.Message, row.GroupName))
	}

	return result, nil
}

func (r *MessageRepository) Update(ctx context.Context, id, title, body string, groupID *string, updatedBy string) error {
	return r.q.UpdateMessage(ctx, db.UpdateMessageParams{
		ID:            id,
		Title:         title,
		Body:          body,
		GroupID:       textFromPtr(groupID),
		LastUpdatedBy: pgtype.Text{String: updatedBy, Valid: true},
	})
}

func (r *MessageRepository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteMessage(ctx, id)
}

func dbMessageToEntity(m db.Message, groupName pgtype.Text) *entity.Message {
	e := &entity.Message{
		ID:            m.ID,
		Title:         m.Title,
		Body:          m.Body,
		GroupID:       ptrFromText(m.GroupID),
		GroupName:     ptrFromText(groupName),
		CreatedBy:     ptrFromText(m.CreatedBy),
		LastUpdatedBy: ptrFromText(m.LastUpdatedBy),
	}
	if m.CreatedAt.Valid {
		e.CreatedAt = m.CreatedAt.Time
	}

	if m.UpdatedAt.Valid {
		e.UpdatedAt = m.UpdatedAt.Time
	}

	return e
}
