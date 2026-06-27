package triggers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

func init() {
	scheduled_op.RegisterTrigger(entity.ScheduledTriggerType_TIME, decodeTimeTrigger)
}

// TimeTrigger は指定時刻以降に ready になる単純 trigger.
// next_fire_at == ScheduledAt なので、worker が当該行を拾った時点で必ず ready.
type TimeTrigger struct {
	ScheduledAt time.Time `json:"scheduled_at"`
}

func NewTimeTrigger(at time.Time) *TimeTrigger {
	return &TimeTrigger{ScheduledAt: at}
}

func (t *TimeTrigger) Type() entity.ScheduledTriggerType {
	return entity.ScheduledTriggerType_TIME
}

func (t *TimeTrigger) Evaluate(_ context.Context, deps scheduled_op.TriggerEvalDeps) (bool, time.Time, error) {
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}

	if now.Before(t.ScheduledAt) {
		return false, t.ScheduledAt, nil
	}

	return true, time.Time{}, nil
}

func (t *TimeTrigger) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeTimeTrigger(cfg json.RawMessage) (scheduled_op.Trigger, error) {
	t := &TimeTrigger{}
	if err := json.Unmarshal(cfg, t); err != nil {
		return nil, errors.WrapPrefix(err, "time trigger", 0)
	}

	if t.ScheduledAt.IsZero() {
		return nil, errors.New("time trigger: scheduled_at is required")
	}

	return t, nil
}
