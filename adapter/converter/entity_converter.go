package converter

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
)

func HeadlessHostEntityToProto(e *entity.HeadlessHost) *hdlctrlv1.HeadlessHost {
	return &hdlctrlv1.HeadlessHost{
		Id:      e.ID,
		Name:    e.Name,
		Address: e.Address,
	}
}
