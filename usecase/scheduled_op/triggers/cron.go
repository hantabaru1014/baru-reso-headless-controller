package triggers

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	// コンテナイメージに zoneinfo が無くても LoadLocation できるよう埋め込む.
	_ "time/tzdata"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op"
)

func init() {
	scheduled_op.RegisterTrigger(entity.ScheduledTriggerType_CRON, decodeCronTrigger)
}

type CronFrequency int32

const (
	CronFrequency_DAILY   CronFrequency = 1
	CronFrequency_WEEKLY  CronFrequency = 2
	CronFrequency_MONTHLY CronFrequency = 3
	CronFrequency_YEARLY  CronFrequency = 4
)

// cronSearchDays は Next が候補日を探す上限. 2/29 指定の年次でも次の閏年
// (最大 8 年先: 2096 → 2104) までに見つかる日数にしておく.
const cronSearchDays = 366 * 9

// CronTrigger は日次・週次・月次・年次で繰り返し発火する trigger.
// 時刻は Timezone 基準で解釈する. 指定日が存在しない月 (31 日, 2/29 など) はスキップする.
type CronTrigger struct {
	Frequency CronFrequency `json:"frequency"`
	Hour      int32         `json:"hour"`
	Minute    int32         `json:"minute"`
	// Weekdays は WEEKLY 用 (0=日曜 ... 6=土曜).
	Weekdays []int32 `json:"weekdays,omitempty"`
	// DayOfMonth は MONTHLY / YEARLY 用 (1-31).
	DayOfMonth int32 `json:"day_of_month,omitempty"`
	// Month は YEARLY 用 (1-12).
	Month int32 `json:"month,omitempty"`
	// Timezone は IANA 名. 空なら UTC.
	Timezone string `json:"timezone,omitempty"`

	// loc は Timezone を解決したもの. NewCronTrigger で 1 度だけ読み込む.
	loc *time.Location
}

var _ scheduled_op.RecurringTrigger = (*CronTrigger)(nil)

// NewCronTrigger は設定を検証して CronTrigger を返す.
// 頻度に関係のないフィールド (日次の曜日など) は捨てる.
func NewCronTrigger(t CronTrigger) (*CronTrigger, error) {
	if err := t.validate(); err != nil {
		return nil, err
	}

	if t.Frequency != CronFrequency_WEEKLY {
		t.Weekdays = nil
	}

	if t.Frequency != CronFrequency_MONTHLY && t.Frequency != CronFrequency_YEARLY {
		t.DayOfMonth = 0
	}

	if t.Frequency != CronFrequency_YEARLY {
		t.Month = 0
	}

	t.loc = time.UTC

	if t.Timezone != "" {
		loc, err := time.LoadLocation(t.Timezone)
		if err != nil {
			return nil, errors.WrapPrefix(err, "cron trigger: invalid timezone", 0)
		}

		t.loc = loc
	}

	return &t, nil
}

func (t *CronTrigger) Type() entity.ScheduledTriggerType {
	return entity.ScheduledTriggerType_CRON
}

// Evaluate は常に ready を返す. claim された時点で発火時刻に達しているため.
// nextCheck には次回の発火時刻を返し、登録時の next_fire_at に使われる.
func (t *CronTrigger) Evaluate(_ context.Context, deps scheduled_op.TriggerEvalDeps) (bool, time.Time, error) {
	now := time.Now()
	if deps.Now != nil {
		now = deps.Now()
	}

	next, err := t.Next(now)
	if err != nil {
		return false, time.Time{}, err
	}

	return true, next, nil
}

func (t *CronTrigger) Next(after time.Time) (time.Time, error) {
	loc := t.loc
	base := after.In(loc)

	for i := range cronSearchDays {
		day := time.Date(base.Year(), base.Month(), base.Day()+i, 0, 0, 0, 0, loc)
		if !t.matchesDay(day) {
			continue
		}

		candidate := time.Date(day.Year(), day.Month(), day.Day(), int(t.Hour), int(t.Minute), 0, 0, loc)
		if candidate.After(after) {
			return candidate, nil
		}
	}

	return time.Time{}, errors.New("cron trigger: no upcoming fire time")
}

func (t *CronTrigger) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(t)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return b, nil
}

func (t *CronTrigger) matchesDay(day time.Time) bool {
	switch t.Frequency {
	case CronFrequency_DAILY:
		return true
	case CronFrequency_WEEKLY:
		return slices.Contains(t.Weekdays, int32(day.Weekday())) //nolint:gosec // Weekday は 0-6
	case CronFrequency_MONTHLY:
		return day.Day() == int(t.DayOfMonth)
	case CronFrequency_YEARLY:
		return day.Month() == time.Month(t.Month) && day.Day() == int(t.DayOfMonth)
	default:
		return false
	}
}

func (t *CronTrigger) validate() error {
	if t.Hour < 0 || t.Hour > 23 {
		return errors.New("cron trigger: hour must be 0-23")
	}

	if t.Minute < 0 || t.Minute > 59 {
		return errors.New("cron trigger: minute must be 0-59")
	}

	switch t.Frequency {
	case CronFrequency_DAILY:
		return nil
	case CronFrequency_WEEKLY:
		if len(t.Weekdays) == 0 {
			return errors.New("cron trigger: weekdays is required for weekly")
		}

		for _, w := range t.Weekdays {
			if w < 0 || w > 6 {
				return errors.New("cron trigger: weekdays must be 0-6")
			}
		}

		return nil
	case CronFrequency_MONTHLY:
		if t.DayOfMonth < 1 || t.DayOfMonth > 31 {
			return errors.New("cron trigger: day_of_month must be 1-31")
		}

		return nil
	case CronFrequency_YEARLY:
		if t.Month < 1 || t.Month > 12 {
			return errors.New("cron trigger: month must be 1-12")
		}

		// 閏年 (2000 年) の日数で判定し、2/29 は許可・2/30 や 4/31 は拒否する.
		daysInMonth := time.Date(2000, time.Month(t.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if t.DayOfMonth < 1 || int(t.DayOfMonth) > daysInMonth {
			return errors.New("cron trigger: day_of_month does not exist in the month")
		}

		return nil
	default:
		return errors.Errorf("cron trigger: invalid frequency %d", t.Frequency)
	}
}

func decodeCronTrigger(cfg json.RawMessage) (scheduled_op.Trigger, error) {
	t := CronTrigger{}
	if err := json.Unmarshal(cfg, &t); err != nil {
		return nil, errors.WrapPrefix(err, "cron trigger", 0)
	}

	return NewCronTrigger(t)
}
