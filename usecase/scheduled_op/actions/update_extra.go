package actions

import (
	"context"
	"encoding/json"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

func init() {
	scheduled_op.RegisterAction(entity.ScheduledOperationType_UPDATE_EXTRA_SETTINGS, decodeUpdateExtra)
}

type UpdateExtraSettingsAction struct {
	SessionID   string  `json:"session_id"`
	AutoUpgrade *bool   `json:"auto_upgrade,omitempty"`
	Memo        *string `json:"memo,omitempty"`
}

func NewUpdateExtraSettingsAction(sessionID string, autoUpgrade *bool, memo *string) *UpdateExtraSettingsAction {
	return &UpdateExtraSettingsAction{SessionID: sessionID, AutoUpgrade: autoUpgrade, Memo: memo}
}

func (a *UpdateExtraSettingsAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_UPDATE_EXTRA_SETTINGS
}

func (a *UpdateExtraSettingsAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	return deps.Session.UpdateSessionExtraSettings(ctx, a.SessionID, a.AutoUpgrade, a.Memo)
}

func (a *UpdateExtraSettingsAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeUpdateExtra(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &UpdateExtraSettingsAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "update extra settings action", 0)
	}

	if a.SessionID == "" {
		return nil, errors.New("update extra settings action: session_id is required")
	}

	if a.AutoUpgrade == nil && a.Memo == nil {
		return nil, errors.New("update extra settings action: at least one field is required")
	}

	return a, nil
}
