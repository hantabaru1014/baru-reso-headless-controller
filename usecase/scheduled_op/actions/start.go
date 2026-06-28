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
	scheduled_op.RegisterAction(entity.ScheduledOperationType_START_SESSION, decodeStartSession)
}

// StartSessionAction は START_SESSION 予約用. payload は WorldStartupParameters の protojson.
type StartSessionAction struct {
	HostID            string          `json:"host_id"`
	GroupID           string          `json:"group_id"`
	UserID            *string         `json:"user_id,omitempty"`
	Memo              *string         `json:"memo,omitempty"`
	StartupParamsJSON json.RawMessage `json:"startup_parameters"`
}

func NewStartSessionAction(hostID string, groupID string, userID *string, memo *string, params json.RawMessage) *StartSessionAction {
	return &StartSessionAction{
		HostID:            hostID,
		GroupID:           groupID,
		UserID:            userID,
		Memo:              memo,
		StartupParamsJSON: params,
	}
}

func (a *StartSessionAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_START_SESSION
}

func (a *StartSessionAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	params := &headlessv1.WorldStartupParameters{}
	if len(a.StartupParamsJSON) > 0 {
		if err := protojson.Unmarshal(a.StartupParamsJSON, params); err != nil {
			return errors.WrapPrefix(err, "decode startup parameters", 0)
		}
	}

	_, err := deps.Session.StartSession(ctx, a.HostID, a.GroupID, a.UserID, params, a.Memo)

	return err
}

func (a *StartSessionAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeStartSession(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &StartSessionAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "start session action", 0)
	}

	if a.HostID == "" {
		return nil, errors.New("start session action: host_id is required")
	}

	return a, nil
}
