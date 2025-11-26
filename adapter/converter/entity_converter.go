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
		HostSettings:     HeadlessHostSettingsToProto(&e.HostSettings),
		Memo:             e.Memo,
		InstanceId:       e.InstanceId,
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

func HeadlessHostSettingsToStartupConfigProto(e *entity.HeadlessHostSettings) *headlessv1.StartupConfig {
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
	return &headlessv1.StartupConfig{
		UniverseId:                  e.UniverseID,
		TickRate:                    &e.TickRate,
		MaxConcurrentAssetTransfers: &e.MaxConcurrentAssetTransfers,
		UsernameOverride:            e.UsernameOverride,
		AllowedUrlHosts:             allowedUrlHosts,
		StartWorlds:                 e.StartWorlds,
	}
}

func HeadlessHostSettingsProtoToEntity(proto *headlessv1.StartupConfig) *entity.HeadlessHostSettings {
	allowedUrlHosts := make([]entity.HostAllowedAccessEntry, 0, len(proto.AllowedUrlHosts))
	for _, entry := range proto.AllowedUrlHosts {
		types := make([]entity.HostAllowedAccessType, 0, len(entry.AccessTypes))
		for _, accessType := range entry.AccessTypes {
			types = append(types, entity.HostAllowedAccessType(accessType))
		}
		allowedUrlHosts = append(allowedUrlHosts, entity.HostAllowedAccessEntry{
			Host:        entry.Host,
			Ports:       entry.Ports,
			AccessTypes: types,
		})
	}
	tickRate := float32(0)
	if proto.TickRate != nil {
		tickRate = *proto.TickRate
	}
	maxConcurrentAssetTransfers := int32(0)
	if proto.MaxConcurrentAssetTransfers != nil {
		maxConcurrentAssetTransfers = *proto.MaxConcurrentAssetTransfers
	}
	return &entity.HeadlessHostSettings{
		UniverseID:                  proto.UniverseId,
		TickRate:                    tickRate,
		MaxConcurrentAssetTransfers: maxConcurrentAssetTransfers,
		UsernameOverride:            proto.UsernameOverride,
		AllowedUrlHosts:             allowedUrlHosts,
		StartWorlds:                 proto.StartWorlds,
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
