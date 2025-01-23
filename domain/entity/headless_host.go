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

type HeadlessHost struct {
	ID                string
	Name              string
	Status            HeadlessHostStatus
	Address           string
	ResoniteVersion   string
	AccountId         string
	AccountName       string
	StorageQuotaBytes int64
	StorageUsedBytes  int64
	Fps               float32
}

type HeadlessHostList []*HeadlessHost
