package actions

import (
	"context"
	"encoding/json"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	scheduled_op.RegisterAction(entity.ScheduledOperationType_UPDATE_PARAMETERS, decodeUpdateParameters)
}

// UpdateParametersAction は UPDATE_PARAMETERS 予約用.
// ParamsJSON は hdlctrl.UpdateSessionParametersRequest の protojson.
type UpdateParametersAction struct {
	SessionID  string          `json:"session_id"`
	ParamsJSON json.RawMessage `json:"params"`
}

func NewUpdateParametersAction(sessionID string, params json.RawMessage) *UpdateParametersAction {
	return &UpdateParametersAction{SessionID: sessionID, ParamsJSON: params}
}

func (a *UpdateParametersAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_UPDATE_PARAMETERS
}

func (a *UpdateParametersAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	req := &headlessv1.UpdateSessionParametersRequest{}
	if err := protojson.Unmarshal(a.ParamsJSON, req); err != nil {
		return errors.WrapPrefix(err, "decode update parameters request", 0)
	}

	req.SessionId = a.SessionID

	return deps.Session.UpdateSessionParameters(ctx, a.SessionID, req)
}

func (a *UpdateParametersAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeUpdateParameters(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &UpdateParametersAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "update parameters action", 0)
	}

	if a.SessionID == "" {
		return nil, errors.New("update parameters action: session_id is required")
	}

	return a, nil
}
