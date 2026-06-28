package entity

import (
	"encoding/json"
	"time"
)

type AsyncJobType int32

const (
	AsyncJobType_UNKNOWN          AsyncJobType = 0
	AsyncJobType_START_HOST       AsyncJobType = 1
	AsyncJobType_SHUTDOWN_HOST    AsyncJobType = 2
	AsyncJobType_RESTART_HOST     AsyncJobType = 3
	AsyncJobType_START_SESSION    AsyncJobType = 4
	AsyncJobType_STOP_SESSION     AsyncJobType = 5
)

type AsyncJobStatus int32

const (
	AsyncJobStatus_PENDING   AsyncJobStatus = 0
	AsyncJobStatus_RUNNING   AsyncJobStatus = 1
	AsyncJobStatus_SUCCEEDED AsyncJobStatus = 2
	AsyncJobStatus_FAILED    AsyncJobStatus = 3
)

type AsyncJob struct {
	ID            string
	JobType       AsyncJobType
	Payload       json.RawMessage
	Status        AsyncJobStatus
	ResultPayload json.RawMessage
	LastError     *string
	ClaimedBy     *string
	ClaimedAt     *time.Time
	ExecutedAt    *time.Time
	HostID        *string
	SessionID     *string
	CreatedBy     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AsyncJobList []*AsyncJob
