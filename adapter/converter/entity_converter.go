package converter

import (
	"context"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	hdlctrlv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
)

func HeadlessHostEntityToProto(e *entity.HeadlessHost) *hdlctrlv1.HeadlessHost {
	return &hdlctrlv1.HeadlessHost{
		Id:      e.ID,
		Name:    e.Name,
		Address: e.Address,
	}
}

func HeadlessHostEntityToProtoFull(ctx context.Context, e *entity.HeadlessHost, conn headlessv1.HeadlessControlServiceClient) (*hdlctrlv1.HeadlessHost, error) {
	aboutResult, err := conn.GetAbout(ctx, &headlessv1.GetAboutRequest{})
	if err != nil {
		return nil, err
	}
	statusResult, err := conn.GetStatus(ctx, &headlessv1.GetStatusRequest{})
	if err != nil {
		return nil, err
	}
	accountInfoResult, err := conn.GetAccountInfo(ctx, &headlessv1.GetAccountInfoRequest{})
	if err != nil {
		return nil, err
	}

	return &hdlctrlv1.HeadlessHost{
		Id:                e.ID,
		Name:              e.Name,
		Address:           e.Address,
		ResoniteVersion:   aboutResult.ResoniteVersion,
		AccountId:         accountInfoResult.UserId,
		AccountName:       accountInfoResult.DisplayName,
		Fps:               statusResult.Fps,
		StorageQuotaBytes: accountInfoResult.StorageQuotaBytes,
		StorageUsedBytes:  accountInfoResult.StorageUsedBytes,
	}, nil
}
