package entity

import (
	"encoding/json"
	"time"
)

type ScheduledOperationType int32

const (
	ScheduledOperationType_UNKNOWN               ScheduledOperationType = 0
	ScheduledOperationType_START_SESSION         ScheduledOperationType = 1
	ScheduledOperationType_STOP_SESSION          ScheduledOperationType = 2
	ScheduledOperationType_UPDATE_PARAMETERS     ScheduledOperationType = 3
	ScheduledOperationType_UPDATE_EXTRA_SETTINGS ScheduledOperationType = 4
)

type ScheduledTriggerType int32

const (
	ScheduledTriggerType_UNKNOWN            ScheduledTriggerType = 0
	ScheduledTriggerType_TIME               ScheduledTriggerType = 1
	ScheduledTriggerType_SESSION_USER_COUNT ScheduledTriggerType = 2
)

type ScheduledOperationStatus int32

const (
	ScheduledOperationStatus_PENDING   ScheduledOperationStatus = 0
	ScheduledOperationStatus_RUNNING   ScheduledOperationStatus = 1
	ScheduledOperationStatus_SUCCEEDED ScheduledOperationStatus = 2
	ScheduledOperationStatus_FAILED    ScheduledOperationStatus = 3
	ScheduledOperationStatus_CANCELED  ScheduledOperationStatus = 4
)

type ScheduledSessionOperation struct {
	ID               string
	OperationType    ScheduledOperationType
	OperationPayload json.RawMessage
	TriggerType      ScheduledTriggerType
	TriggerConfig    json.RawMessage
	NextFireAt       time.Time
	HostID           *string
	SessionID        *string
	Status           ScheduledOperationStatus
	LastError        *string
	ClaimedBy        *string
	ClaimedAt        *time.Time
	ExecutedAt       *time.Time
	CreatedBy        *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ScheduledSessionOperationList []*ScheduledSessionOperation
