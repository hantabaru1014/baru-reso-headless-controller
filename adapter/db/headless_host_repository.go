package db

import (
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

var _ port.HeadlessHostRepository = (*HeadlessHostRepository)(nil)

type HeadlessHostRepository struct {
}

var dummyHost = &entity.HeadlessHost{
	ID:      "1",
	Name:    os.Getenv("DUMMY_HOST_NAME"),
	Address: os.Getenv("DUMMY_HOST_ADDRESS"),
}

// Find implements repository.HeadlessHostRepository.
func (h *HeadlessHostRepository) Find(id string) (*entity.HeadlessHost, error) {
	if id != dummyHost.ID {
		return nil, nil
	}
	return dummyHost, nil
}

// ListAll implements repository.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListAll() (entity.HeadlessHostList, error) {
	dummyList := entity.HeadlessHostList{
		dummyHost,
	}
	return dummyList, nil
}

func NewHeadlessHostRepository() *HeadlessHostRepository {
	return &HeadlessHostRepository{}
}
