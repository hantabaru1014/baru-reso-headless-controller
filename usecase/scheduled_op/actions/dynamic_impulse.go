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
	scheduled_op.RegisterAction(entity.ScheduledOperationType_SEND_DYNAMIC_IMPULSE, entity.PermKey_SessionWrite, decodeSendDynamicImpulse)
}

// SendDynamicImpulseAction はセッションに dynamic impulse を送出する.
// ParamsJSON は headless.v1.SendDynamicImpulseRequest の protojson. session_id は SessionID を正とする.
type SendDynamicImpulseAction struct {
	SessionID  string          `json:"session_id"`
	ParamsJSON json.RawMessage `json:"params"`
}

func NewSendDynamicImpulseAction(sessionID string, params json.RawMessage) *SendDynamicImpulseAction {
	return &SendDynamicImpulseAction{SessionID: sessionID, ParamsJSON: params}
}

func (a *SendDynamicImpulseAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_SEND_DYNAMIC_IMPULSE
}

func (a *SendDynamicImpulseAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	req, err := a.Request()
	if err != nil {
		return err
	}

	return deps.Session.SendDynamicImpulse(ctx, req)
}

// Request は payload を headless の SendDynamicImpulseRequest に復元する.
func (a *SendDynamicImpulseAction) Request() (*headlessv1.SendDynamicImpulseRequest, error) {
	req := &headlessv1.SendDynamicImpulseRequest{}
	if err := protojson.Unmarshal(a.ParamsJSON, req); err != nil {
		return nil, errors.WrapPrefix(err, "decode send dynamic impulse request", 0)
	}

	req.SessionId = a.SessionID

	return req, nil
}

func (a *SendDynamicImpulseAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeSendDynamicImpulse(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &SendDynamicImpulseAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "send dynamic impulse action", 0)
	}

	if a.SessionID == "" {
		return nil, errors.New("send dynamic impulse action: session_id is required")
	}

	return a, nil
}
