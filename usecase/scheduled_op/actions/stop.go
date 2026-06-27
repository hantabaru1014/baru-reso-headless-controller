package actions

import (
	"context"
	"encoding/json"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

func init() {
	scheduled_op.RegisterAction(entity.ScheduledOperationType_STOP_SESSION, decodeStopSession)
}

type StopSessionAction struct {
	SessionID string `json:"session_id"`
}

func NewStopSessionAction(sessionID string) *StopSessionAction {
	return &StopSessionAction{SessionID: sessionID}
}

func (a *StopSessionAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_STOP_SESSION
}

func (a *StopSessionAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	return deps.Session.StopSession(ctx, a.SessionID)
}

func (a *StopSessionAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeStopSession(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &StopSessionAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "stop session action", 0)
	}

	if a.SessionID == "" {
		return nil, errors.New("stop session action: session_id is required")
	}

	return a, nil
}
