// Package scheduled_op provides the Trigger / Action abstraction backing
// scheduled session operations. The executor (worker/scheduled_operation_executor)
// is intentionally agnostic to specific trigger or action kinds — they plug in
// via the registries here, so future trigger types (e.g. "user_count == 0")
// can be added without touching executor or DB schema.
package scheduled_op

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

// TriggerEvalDeps は Trigger.Evaluate に渡される依存。後続 PR の condition 系 trigger
// (例: user 数 == 0) で session の現状を参照する用途。Time trigger では未使用。
type TriggerEvalDeps struct {
	Now         func() time.Time
	SessionRepo port.SessionRepository
	StateCache  port.SessionStateCache
}

type Trigger interface {
	Type() entity.ScheduledTriggerType
	// Evaluate は trigger の発火可否を判定する.
	//   ready=true                  → executor は Action を実行する
	//   ready=false, nextCheck=t    → executor は next_fire_at を t に更新し PENDING に戻す
	Evaluate(ctx context.Context, deps TriggerEvalDeps) (ready bool, nextCheck time.Time, err error)
	Marshal() (json.RawMessage, error)
}

type TriggerFactory func(json.RawMessage) (Trigger, error)

var (
	triggerRegistryMu sync.RWMutex
	triggerRegistry   = map[entity.ScheduledTriggerType]TriggerFactory{}
)

// RegisterTrigger は trigger 種別ごとの factory を登録する.
// 各 trigger 実装の init() から呼ぶ.
func RegisterTrigger(t entity.ScheduledTriggerType, f TriggerFactory) {
	triggerRegistryMu.Lock()
	defer triggerRegistryMu.Unlock()

	if _, exists := triggerRegistry[t]; exists {
		panic("trigger factory already registered for type")
	}

	triggerRegistry[t] = f
}

func DecodeTrigger(t entity.ScheduledTriggerType, cfg json.RawMessage) (Trigger, error) {
	triggerRegistryMu.RLock()

	f, ok := triggerRegistry[t]

	triggerRegistryMu.RUnlock()

	if !ok {
		return nil, errors.Errorf("unknown trigger type: %d", t)
	}

	return f(cfg)
}
