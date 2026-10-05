package scheduled_op

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// SessionOperator は Action が実行時に呼ぶセッション操作の最小集合.
// usecase.SessionUsecase のメソッドそのままを取る形にして、ラッパ層を挟まない.
// 各 Action は payload (protojson 等) を自前で decode してこの interface を呼ぶ.
type SessionOperator interface {
	StartSession(ctx context.Context, hostID string, groupID string, userID *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error)
	StopSession(ctx context.Context, sessionID string) error
	UpdateSessionParameters(ctx context.Context, sessionID string, params *headlessv1.UpdateSessionParametersRequest) error
	UpdateSessionExtraSettings(ctx context.Context, sessionID string, autoUpgrade *bool, memo *string) error
	SendDynamicImpulse(ctx context.Context, req *headlessv1.SendDynamicImpulseRequest) error
}

// HostOperator は Action が実行時に呼ぶホスト操作の最小集合.
// ホストの起動・停止は時間がかかるため、実装は非同期 job として投入して即座に返す.
type HostOperator interface {
	// StartStoppedHost は停止中のホストなら起動 job を投入して true を返す. 停止中でなければ何もしない.
	StartStoppedHost(ctx context.Context, hostID string, withWorldRestart bool) (bool, error)
	RestartHost(ctx context.Context, req *hdlctrlv1.RestartHeadlessHostRequest) error
	ShutdownHost(ctx context.Context, hostID string) error
}

type ActionExecDeps struct {
	Session SessionOperator
	Host    HostOperator
}

type Action interface {
	Type() entity.ScheduledOperationType
	Execute(ctx context.Context, deps ActionExecDeps) error
	Marshal() (json.RawMessage, error)
}

type ActionFactory func(json.RawMessage) (Action, error)

type actionRegistration struct {
	factory ActionFactory
	// permKey は予約の登録・キャンセル時に対象 host/session の group に要求する permission key.
	// 実行時の権限は Execute が呼ぶ usecase 側で改めて検査される.
	permKey string
}

var (
	actionRegistryMu sync.RWMutex
	actionRegistry   = map[entity.ScheduledOperationType]actionRegistration{}
)

// RegisterAction は action 種別ごとの factory と、予約に必要な permission key を登録する.
// 各 action 実装の init() から呼ぶ.
func RegisterAction(t entity.ScheduledOperationType, permKey string, f ActionFactory) {
	actionRegistryMu.Lock()
	defer actionRegistryMu.Unlock()

	if _, exists := actionRegistry[t]; exists {
		panic("action factory already registered for type")
	}

	actionRegistry[t] = actionRegistration{factory: f, permKey: permKey}
}

func lookupAction(t entity.ScheduledOperationType) (actionRegistration, error) {
	actionRegistryMu.RLock()
	defer actionRegistryMu.RUnlock()

	r, ok := actionRegistry[t]
	if !ok {
		return actionRegistration{}, errors.Errorf("unknown action type: %d", t)
	}

	return r, nil
}

func DecodeAction(t entity.ScheduledOperationType, payload json.RawMessage) (Action, error) {
	r, err := lookupAction(t)
	if err != nil {
		return nil, err
	}

	return r.factory(payload)
}

// RequiredPermission は action 種別の予約に必要な permission key を返す.
func RequiredPermission(t entity.ScheduledOperationType) (string, error) {
	r, err := lookupAction(t)
	if err != nil {
		return "", err
	}

	return r.permKey, nil
}
