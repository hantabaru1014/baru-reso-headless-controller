package converter

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func HeadlessHostEntityToProto(e *entity.HeadlessHost) *hdlctrlv1.HeadlessHost {
	return &hdlctrlv1.HeadlessHost{
		Id:               e.ID,
		Name:             e.Name,
		ResoniteVersion:  e.ResoniteVersion,
		AppVersion:       e.AppVersion,
		AccountId:        e.AccountId,
		AccountName:      e.AccountName,
		Fps:              e.Fps,
		Status:           hdlctrlv1.HeadlessHostStatus(e.Status),
		AutoUpdatePolicy: hdlctrlv1.HeadlessHostAutoUpdatePolicy(e.AutoUpdatePolicy),
		Memo:             e.Memo,
	}
}

func HeadlessHostSettingsToProto(e *entity.HeadlessHostSettings) *hdlctrlv1.HeadlessHostSettings {
	allowedUrlHosts := make([]*headlessv1.AllowedAccessEntry, 0, len(e.AllowedUrlHosts))
	for _, entry := range e.AllowedUrlHosts {
		types := make([]headlessv1.AllowedAccessEntry_AccessType, 0, len(entry.AccessTypes))
		for _, accessType := range entry.AccessTypes {
			types = append(types, headlessv1.AllowedAccessEntry_AccessType(accessType))
		}
		allowedUrlHosts = append(allowedUrlHosts, &headlessv1.AllowedAccessEntry{
			Host:        entry.Host,
			Ports:       entry.Ports,
			AccessTypes: types,
		})
	}
	return &hdlctrlv1.HeadlessHostSettings{
		UniverseId:                  e.UniverseID,
		TickRate:                    e.TickRate,
		MaxConcurrentAssetTransfers: e.MaxConcurrentAssetTransfers,
		UsernameOverride:            e.UsernameOverride,
		AllowedUrlHosts:             allowedUrlHosts,
		AutoSpawnItems:              e.AutoSpawnItems,
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
		OwnerId:           e.OwnerID,
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
