package entity

import (
	"time"

	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

type SessionStatus int32

const (
	SessionStatus_UNKNOWN  SessionStatus = 0
	SessionStatus_STARTING SessionStatus = 1
	SessionStatus_RUNNING  SessionStatus = 2
	SessionStatus_ENDED    SessionStatus = 3
	SessionStatus_CRASHED  SessionStatus = 4
)

type Session struct {
	ID                string
	Name              string
	Status            SessionStatus
	StartedAt         *time.Time
	StartedBy         *string
	EndedAt           *time.Time
	HostID            string
	StartupParameters *headlessv1.WorldStartupParameters
	AutoUpgrade       bool
	Memo              string
	CurrentState      *headlessv1.Session
}

type SessionList []*Session
