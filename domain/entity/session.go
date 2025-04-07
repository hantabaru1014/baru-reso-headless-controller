package entity

import (
	"time"

	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (s *Session) ToProto() *hdlctrlv1.Session {
	d := &hdlctrlv1.Session{
		Id:                s.ID,
		Name:              s.Name,
		HostId:            s.HostID,
		Status:            hdlctrlv1.SessionStatus(s.Status),
		StartupParameters: s.StartupParameters,
		CurrentState:      s.CurrentState,
		StartedBy:         s.StartedBy,
		AutoUpgrade:       s.AutoUpgrade,
		Memo:              s.Memo,
	}
	if s.StartedAt != nil {
		d.StartedAt = timestamppb.New(*s.StartedAt)
	}
	if s.EndedAt != nil {
		d.EndedAt = timestamppb.New(*s.EndedAt)
	}
	return d
}

type SessionList []*Session
