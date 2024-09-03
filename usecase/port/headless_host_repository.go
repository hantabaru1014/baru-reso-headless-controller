package port

import "github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"

type HeadlessHostRepository interface {
	ListAll() (entity.HeadlessHostList, error)
	Find(id string) (*entity.HeadlessHost, error)
}
