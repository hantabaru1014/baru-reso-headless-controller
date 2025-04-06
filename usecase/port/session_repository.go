package port

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

type SessionRepository interface {
	Upsert(ctx context.Context, session *entity.Session) error
	UpdateStatus(ctx context.Context, id string, status entity.SessionStatus) error
	Get(ctx context.Context, id string) (*entity.Session, error)
	ListAll(ctx context.Context) (entity.SessionList, error)
	ListByStatus(ctx context.Context, status entity.SessionStatus) (entity.SessionList, error)
	Delete(ctx context.Context, id string) error
}
