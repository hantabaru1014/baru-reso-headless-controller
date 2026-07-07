package port

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

// MessageRepository はお知らせメッセージ (掲示板) の永続化を担う.
type MessageRepository interface {
	// Create は新しいメッセージを作成する. groupID が nil なら全員向け.
	// last_updated_by は created_by と同値で初期化される.
	Create(ctx context.Context, id, title, body string, groupID *string, createdBy string) (*entity.Message, error)
	// Get は 1 件取得する. group_name は解決済み.
	Get(ctx context.Context, id string) (*entity.Message, error)
	// ListAll は全メッセージを updated_at 降順で返す (system:message.manage 保持者向け).
	ListAll(ctx context.Context) (entity.MessageList, error)
	// ListVisibleToUser は全員向け + userID の所属グループ向けを updated_at 降順で返す.
	ListVisibleToUser(ctx context.Context, userID string) (entity.MessageList, error)
	// Update は title / body / group_id を完全置換する.
	Update(ctx context.Context, id, title, body string, groupID *string, updatedBy string) error
	Delete(ctx context.Context, id string) error
}
