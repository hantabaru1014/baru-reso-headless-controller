package entity

type HeadlessHostStatus int32

const (
	HeadlessHostStatus_UNKNOWN  HeadlessHostStatus = 0
	HeadlessHostStatus_STARTING HeadlessHostStatus = 1
	HeadlessHostStatus_RUNNING  HeadlessHostStatus = 2
	HeadlessHostStatus_STOPPING HeadlessHostStatus = 3
	HeadlessHostStatus_EXITED   HeadlessHostStatus = 4
	HeadlessHostStatus_CRASHED  HeadlessHostStatus = 5
)

type HostAllowedAccessType int32

const (
	HostAllowedAccessType_UNSPECIFIED   HostAllowedAccessType = 0
	HostAllowedAccessType_HTTP          HostAllowedAccessType = 1
	HostAllowedAccessType_WEBSOCKET     HostAllowedAccessType = 2
	HostAllowedAccessType_OSC_RECEIVING HostAllowedAccessType = 3
	HostAllowedAccessType_OSC_SENDING   HostAllowedAccessType = 4
)

type HostAllowedAccessEntry struct {
	Host        string
	Ports       []int32
	AccessTypes []HostAllowedAccessType
}

type HeadlessHostSettings struct {
	UniverseID                  *string
	TickRate                    float32
	MaxConcurrentAssetTransfers int32
	UsernameOverride            *string
	AllowedUrlHosts             []HostAllowedAccessEntry
	AutoSpawnItems              []string
}

type HeadlessHost struct {
	ID                string
	Name              string
	Status            HeadlessHostStatus
	Address           string
	ResoniteVersion   string
	AppVersion        string
	AccountId         string
	AccountName       string
	StorageQuotaBytes int64
	StorageUsedBytes  int64
	Fps               float32
}

type HeadlessHostList []*HeadlessHost
