package entity

type HeadlessHost struct {
	ID                string
	Name              string
	Address           string
	ResoniteVersion   string
	AccountId         string
	AccountName       string
	StorageQuotaBytes int64
	StorageUsedBytes  int64
	Fps               float32
}

type HeadlessHostList []*HeadlessHost
