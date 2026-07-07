package usecase

import (
	"context"

	"github.com/dchest/uniuri"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// ErrMessageTitleRequired は title が空文字のまま作成/更新しようとした.
var ErrMessageTitleRequired = errors.New("message title is required")

// MessageUsecase はお知らせメッセージ (掲示板) の閲覧 + CRUD を提供する.
type MessageUsecase struct {
	repo      port.MessageRepository
	groupRepo port.GroupRepository
	permUC    *PermissionUsecase
}

func NewMessageUsecase(repo port.MessageRepository, groupRepo port.GroupRepository, permUC *PermissionUsecase) *MessageUsecase {
	return &MessageUsecase{
		repo:      repo,
		groupRepo: groupRepo,
		permUC:    permUC,
	}
}

// ListMessages は caller が閲覧可能なメッセージを updated_at 降順で返す.
// system:message.manage 保持者は全件、それ以外は全員向け + 所属グループ向けのみ.
func (u *MessageUsecase) ListMessages(ctx context.Context) (entity.MessageList, error) {
	callerID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	canManage, err := u.permUC.HasSystemPermission(ctx, callerID, entity.PermKey_SystemMessageManage)
	if err != nil {
		return nil, err
	}

	if canManage {
		return u.repo.ListAll(ctx)
	}

	return u.repo.ListVisibleToUser(ctx, callerID)
}

type CreateMessageParams struct {
	Title   string
	Body    string
	GroupID *string
}

// CreateMessage は新しいメッセージを作成する. system:message.manage が必要.
func (u *MessageUsecase) CreateMessage(ctx context.Context, params CreateMessageParams) (*entity.Message, error) {
	// ===== 入力検証 (書き込み前に全部済ませる) =====
	if params.Title == "" {
		return nil, errors.Wrap(ErrMessageTitleRequired, 0)
	}

	if params.GroupID != nil && *params.GroupID != "" {
		if _, err := u.groupRepo.Get(ctx, *params.GroupID); err != nil {
			return nil, err
		}
	}

	// ===== 認可 =====
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemMessageManage); err != nil {
		return nil, err
	}

	callerID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	// ===== 書き込み =====
	id := "msg-" + uniuri.New()

	return u.repo.Create(ctx, id, params.Title, params.Body, normalizeGroupID(params.GroupID), callerID)
}

type UpdateMessageParams struct {
	ID      string
	Title   string
	Body    string
	GroupID *string
}

// UpdateMessage は title / body / 対象グループを完全置換する. system:message.manage が必要.
func (u *MessageUsecase) UpdateMessage(ctx context.Context, params UpdateMessageParams) (*entity.Message, error) {
	// ===== 入力検証 (書き込み前に全部済ませる) =====
	if params.Title == "" {
		return nil, errors.Wrap(ErrMessageTitleRequired, 0)
	}

	if params.GroupID != nil && *params.GroupID != "" {
		if _, err := u.groupRepo.Get(ctx, *params.GroupID); err != nil {
			return nil, err
		}
	}

	// ===== 認可 =====
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemMessageManage); err != nil {
		return nil, err
	}

	callerID, err := CurrentUserID(ctx)
	if err != nil {
		return nil, err
	}

	// ===== 書き込み =====
	if _, err := u.repo.Get(ctx, params.ID); err != nil {
		return nil, err
	}

	if err := u.repo.Update(ctx, params.ID, params.Title, params.Body, normalizeGroupID(params.GroupID), callerID); err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return u.repo.Get(ctx, params.ID)
}

// DeleteMessage はメッセージを削除する. system:message.manage が必要.
func (u *MessageUsecase) DeleteMessage(ctx context.Context, id string) error {
	if err := u.permUC.RequireSystemPermission(ctx, entity.PermKey_SystemMessageManage); err != nil {
		return err
	}

	if _, err := u.repo.Get(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}

		return errors.Wrap(err, 0)
	}

	return u.repo.Delete(ctx, id)
}

// normalizeGroupID は空文字を nil (全員向け) に正規化する.
func normalizeGroupID(groupID *string) *string {
	if groupID == nil || *groupID == "" {
		return nil
	}

	return groupID
}
