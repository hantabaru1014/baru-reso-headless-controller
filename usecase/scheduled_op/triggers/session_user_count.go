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
	scheduled_op.RegisterTrigger(entity.ScheduledTriggerType_SESSION_USER_COUNT, decodeSessionUserCountTrigger)
}

type SessionUserCountComparator int32

const (
	SessionUserCountComparator_LESS_OR_EQUAL    SessionUserCountComparator = 1
	SessionUserCountComparator_GREATER_OR_EQUAL SessionUserCountComparator = 2
)

// sessionUserCountMissBackoff は cache miss 時の次回再評価までの待ち時間.
// 起動前 / event 未到達は数十秒〜分単位で解消されるので、executor の tick 間隔 (10s) で
// 毎回 SessionRepo.Get を打つのは無駄. 1 分ごとに緩めて評価する.
const sessionUserCountMissBackoff = time.Minute

// SessionUserCountTrigger は指定 session のユーザー数が閾値条件を満たした際に ready になる.
// container から流れる SessionParametersChanged event で SessionStateCache が更新される
// たびに worker が再評価する想定 (poll based の現実装では tick interval ごと).
//
// 対象 session が ENDED に至った場合は trigger を失敗として扱う (cache miss + DB ENDED).
// それ以外の cache miss (起動前 / event 未到達) は ready=false で requeue.
type SessionUserCountTrigger struct {
	SessionID  string                     `json:"session_id"`
	Comparator SessionUserCountComparator `json:"comparator"`
	Threshold  int32                      `json:"threshold"`
}

func NewSessionUserCountTrigger(sessionID string, comparator SessionUserCountComparator, threshold int32) *SessionUserCountTrigger {
	return &SessionUserCountTrigger{
		SessionID:  sessionID,
		Comparator: comparator,
		Threshold:  threshold,
	}
}

func (t *SessionUserCountTrigger) Type() entity.ScheduledTriggerType {
	return entity.ScheduledTriggerType_SESSION_USER_COUNT
}

func (t *SessionUserCountTrigger) Evaluate(ctx context.Context, deps scheduled_op.TriggerEvalDeps) (bool, time.Time, error) {
	// StateCache が無いのは usecase.Create からの「初回登録時の next_fire_at 取得」呼び出し。
	// 即時 ready ではない (count を見ていない) ので false + zero を返し、即時 worker に拾わせる.
	if deps.StateCache == nil {
		return false, time.Time{}, nil
	}

	snapshot, ok := deps.StateCache.Get(t.SessionID)
	if !ok {
		// cache miss. 対象 session が既に終端 (ENDED) なら trigger は意味を失うので失敗にする.
		// それ以外 (起動前 / event 未到達) は backoff して requeue.
		if deps.SessionRepo != nil {
			if s, err := deps.SessionRepo.Get(ctx, t.SessionID); err == nil && s != nil {
				if s.Status == entity.SessionStatus_ENDED {
					return false, time.Time{}, errors.Errorf("session %s has ended; trigger will never fire", t.SessionID)
				}
			}
		}

		return false, time.Now().Add(sessionUserCountMissBackoff), nil
	}

	count := snapshot.GetUsersCount()

	switch t.Comparator {
	case SessionUserCountComparator_LESS_OR_EQUAL:
		if count <= t.Threshold {
			return true, time.Time{}, nil
		}
	case SessionUserCountComparator_GREATER_OR_EQUAL:
		if count >= t.Threshold {
			return true, time.Time{}, nil
		}
	default:
		return false, time.Time{}, errors.Errorf("session_user_count trigger: invalid comparator %d", t.Comparator)
	}

	return false, time.Time{}, nil
}

func (t *SessionUserCountTrigger) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func decodeSessionUserCountTrigger(cfg json.RawMessage) (scheduled_op.Trigger, error) {
	t := &SessionUserCountTrigger{}
	if err := json.Unmarshal(cfg, t); err != nil {
		return nil, errors.WrapPrefix(err, "session_user_count trigger", 0)
	}

	if t.SessionID == "" {
		return nil, errors.New("session_user_count trigger: session_id is required")
	}

	switch t.Comparator {
	case SessionUserCountComparator_LESS_OR_EQUAL, SessionUserCountComparator_GREATER_OR_EQUAL:
		// ok
	default:
		return nil, errors.Errorf("session_user_count trigger: invalid comparator %d", t.Comparator)
	}

	if t.Threshold < 0 {
		return nil, errors.New("session_user_count trigger: threshold must be >= 0")
	}

	return t, nil
}
