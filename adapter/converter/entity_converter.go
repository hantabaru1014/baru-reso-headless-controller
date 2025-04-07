package converter

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func HeadlessHostEntityToProto(e *entity.HeadlessHost) *hdlctrlv1.HeadlessHost {
	return &hdlctrlv1.HeadlessHost{
		Id:                e.ID,
		Name:              e.Name,
		Address:           e.Address,
		ResoniteVersion:   e.ResoniteVersion,
		AccountId:         e.AccountId,
		AccountName:       e.AccountName,
		StorageQuotaBytes: e.StorageQuotaBytes,
		StorageUsedBytes:  e.StorageUsedBytes,
		Fps:               e.Fps,
		Status:            hdlctrlv1.HeadlessHostStatus(e.Status),
	}
}

func SessionEntityToProto(e *entity.Session) *hdlctrlv1.Session {
	d := &hdlctrlv1.Session{
		Id:                e.ID,
		Name:              e.Name,
		HostId:            e.HostID,
		Status:            hdlctrlv1.SessionStatus(e.Status),
		StartupParameters: e.StartupParameters,
		CurrentState:      e.CurrentState,
		StartedBy:         e.StartedBy,
		AutoUpgrade:       e.AutoUpgrade,
		Memo:              e.Memo,
	}
	if e.StartedAt != nil {
		d.StartedAt = timestamppb.New(*e.StartedAt)
	}
	if e.EndedAt != nil {
		d.EndedAt = timestamppb.New(*e.EndedAt)
	}
	return d
}
