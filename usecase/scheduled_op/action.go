package scheduled_op

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

// SessionOperator は Action が実行時に呼ぶセッション操作の最小集合.
// usecase.SessionUsecase のメソッドそのままを取る形にして、ラッパ層を挟まない.
// 各 Action は payload (protojson 等) を自前で decode してこの interface を呼ぶ.
type SessionOperator interface {
	StartSession(ctx context.Context, hostID string, userID *string, params *headlessv1.WorldStartupParameters, memo *string) (*entity.Session, error)
	StopSession(ctx context.Context, sessionID string) error
	UpdateSessionParameters(ctx context.Context, sessionID string, params *headlessv1.UpdateSessionParametersRequest) error
	UpdateSessionExtraSettings(ctx context.Context, sessionID string, autoUpgrade *bool, memo *string) error
}

type ActionExecDeps struct {
	Session SessionOperator
}

type Action interface {
	Type() entity.ScheduledOperationType
	Execute(ctx context.Context, deps ActionExecDeps) error
	Marshal() (json.RawMessage, error)
}

type ActionFactory func(json.RawMessage) (Action, error)

var (
	actionRegistryMu sync.RWMutex
	actionRegistry   = map[entity.ScheduledOperationType]ActionFactory{}
)

func RegisterAction(t entity.ScheduledOperationType, f ActionFactory) {
	actionRegistryMu.Lock()
	defer actionRegistryMu.Unlock()

	if _, exists := actionRegistry[t]; exists {
		panic("action factory already registered for type")
	}

	actionRegistry[t] = f
}

func DecodeAction(t entity.ScheduledOperationType, payload json.RawMessage) (Action, error) {
	actionRegistryMu.RLock()

	f, ok := actionRegistry[t]

	actionRegistryMu.RUnlock()

	if !ok {
		return nil, errors.Errorf("unknown action type: %d", t)
	}

	return f(payload)
}
