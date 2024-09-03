package entity

type HeadlessHost struct {
	ID      string
	Name    string
	Address string
}

type HeadlessHostList []*HeadlessHost
