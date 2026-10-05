package actions

import (
	"context"
	"encoding/json"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	scheduled_op.RegisterAction(entity.ScheduledOperationType_START_HOST, entity.PermKey_HostWrite, decodeStartHost)
	scheduled_op.RegisterAction(entity.ScheduledOperationType_RESTART_HOST, entity.PermKey_HostWrite, decodeRestartHost)
	scheduled_op.RegisterAction(entity.ScheduledOperationType_SHUTDOWN_HOST, entity.PermKey_HostWrite, decodeShutdownHost)
}

// StartHostAction は停止中のホストを起動する. 実行時に停止中でなければ何もしない.
type StartHostAction struct {
	HostID           string `json:"host_id"`
	WithWorldRestart bool   `json:"with_world_restart,omitempty"`
}

func NewStartHostAction(hostID string, withWorldRestart bool) *StartHostAction {
	return &StartHostAction{HostID: hostID, WithWorldRestart: withWorldRestart}
}

func (a *StartHostAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_START_HOST
}

func (a *StartHostAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	_, err := deps.Host.StartStoppedHost(ctx, a.HostID, a.WithWorldRestart)

	return err
}

func (a *StartHostAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeStartHost(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &StartHostAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "start host action", 0)
	}

	if a.HostID == "" {
		return nil, errors.New("start host action: host_id is required")
	}

	return a, nil
}

// RestartHostAction はホストを再起動する. 停止中のホストなら起動になる.
// ParamsJSON は hdlctrl.RestartHeadlessHostRequest の protojson.
type RestartHostAction struct {
	HostID     string          `json:"host_id"`
	ParamsJSON json.RawMessage `json:"params"`
}

func NewRestartHostAction(req *hdlctrlv1.RestartHeadlessHostRequest) (*RestartHostAction, error) {
	params, err := protojson.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return &RestartHostAction{HostID: req.GetHostId(), ParamsJSON: params}, nil
}

func (a *RestartHostAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_RESTART_HOST
}

func (a *RestartHostAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	req, err := a.Request()
	if err != nil {
		return err
	}

	return deps.Host.RestartHost(ctx, req)
}

// Request は payload を async job に渡す RestartHeadlessHostRequest に復元する.
func (a *RestartHostAction) Request() (*hdlctrlv1.RestartHeadlessHostRequest, error) {
	req := &hdlctrlv1.RestartHeadlessHostRequest{}
	if err := protojson.Unmarshal(a.ParamsJSON, req); err != nil {
		return nil, errors.WrapPrefix(err, "decode restart host request", 0)
	}

	req.HostId = a.HostID

	return req, nil
}

func (a *RestartHostAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeRestartHost(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &RestartHostAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "restart host action", 0)
	}

	if a.HostID == "" {
		return nil, errors.New("restart host action: host_id is required")
	}

	return a, nil
}

type ShutdownHostAction struct {
	HostID string `json:"host_id"`
}

func NewShutdownHostAction(hostID string) *ShutdownHostAction {
	return &ShutdownHostAction{HostID: hostID}
}

func (a *ShutdownHostAction) Type() entity.ScheduledOperationType {
	return entity.ScheduledOperationType_SHUTDOWN_HOST
}

func (a *ShutdownHostAction) Execute(ctx context.Context, deps scheduled_op.ActionExecDeps) error {
	return deps.Host.ShutdownHost(ctx, a.HostID)
}

func (a *ShutdownHostAction) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeShutdownHost(payload json.RawMessage) (scheduled_op.Action, error) {
	a := &ShutdownHostAction{}
	if err := json.Unmarshal(payload, a); err != nil {
		return nil, errors.WrapPrefix(err, "shutdown host action", 0)
	}

	if a.HostID == "" {
		return nil, errors.New("shutdown host action: host_id is required")
	}

	return a, nil
}
