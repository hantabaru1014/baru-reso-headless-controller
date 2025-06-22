package skyfrost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/go-errors/errors"
)

type StorageInfo struct {
	Id                  string `json:"id"`
	OwnerId             string `json:"ownerId"`
	UsedBytes           int64  `json:"usedBytes"`
	QuotaBytes          int64  `json:"quotaBytes"`
	FullQuotaBytes      int64  `json:"fullQuotaBytes"`
	ShareableQuotaBytes int64  `json:"shareableQuotaBytes"`
}

func (s *UserSession) GetStorage(ctx context.Context, ownerId string) (*StorageInfo, error) {
	reqUrl, err := url.JoinPath(API_BASE_URL, "users", ownerId, "storage")
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %s", err)
	}
	req, err := s.makeApiRequest(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to login: %s", resp.Status)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	storageInfo := &StorageInfo{}
	err = json.Unmarshal(respBody, storageInfo)
	if err != nil {
		return nil, errors.Errorf("failed to parse storage info: %s", err)
	}

	return storageInfo, nil
}
