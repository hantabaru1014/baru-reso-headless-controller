package skyfrost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
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
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}
	req, err := s.makeApiRequest(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to login: %s", resp.Status)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	storageInfo := &StorageInfo{}
	err = json.Unmarshal(respBody, storageInfo)
	if err != nil {
		return nil, errors.Errorf("failed to parse storage info: %w", err)
	}

	return storageInfo, nil
}

// RecordVersion represents version information for a record
type RecordVersion struct {
	GlobalVersion          int    `json:"globalVersion"`
	LocalVersion           int    `json:"localVersion"`
	LastModifyingUserId    string `json:"lastModifyingUserId"`
	LastModifyingMachineId string `json:"lastModifyingMachineId"`
}

// DBAsset represents an asset in the manifest
type DBAsset struct {
	Hash  string `json:"hash"`
	Bytes int64  `json:"bytes"`
}

// Record represents a Resonite record
type Record struct {
	Id                   string        `json:"id"`
	OwnerId              string        `json:"ownerId"`
	AssetUri             string        `json:"assetUri"`
	Version              RecordVersion `json:"version"`
	Name                 string        `json:"name"`
	Description          string        `json:"description,omitempty"`
	RecordType           string        `json:"recordType"`
	OwnerName            string        `json:"ownerName,omitempty"`
	Tags                 []string      `json:"tags,omitempty"`
	Path                 string        `json:"path"`
	ThumbnailUri         string        `json:"thumbnailUri,omitempty"`
	LastModificationTime string        `json:"lastModificationTime"`
	CreationTime         string        `json:"creationTime,omitempty"`
	IsDeleted            bool          `json:"isDeleted"`
	IsPublic             bool          `json:"isPublic"`
	IsForPatrons         bool          `json:"isForPatrons"`
	IsListed             bool          `json:"isListed"`
	IsReadOnly           bool          `json:"isReadOnly"`
	Visits               int           `json:"visits"`
	Rating               float64       `json:"rating"`
	RandomOrder          int           `json:"randomOrder"`
	AssetManifest        []DBAsset     `json:"assetManifest,omitempty"`
}

// RecordPreprocessStatus represents the status of record preprocessing
type RecordPreprocessStatus struct {
	Id          string      `json:"id"`
	OwnerId     string      `json:"ownerId"`
	RecordId    string      `json:"recordId"`
	State       string      `json:"state"` // "Preprocessing", "Finished", "Failed"
	Progress    float64     `json:"progress"`
	FailReason  string      `json:"failReason,omitempty"`
	ResultDiffs []AssetDiff `json:"resultDiffs,omitempty"`
}

// AssetDiffState represents the state of an asset diff
type AssetDiffState int

const (
	AssetDiffStateAdded     AssetDiffState = 0
	AssetDiffStateUnchanged AssetDiffState = 1
	AssetDiffStateRemoved   AssetDiffState = 2
)

// AssetDiff represents the difference status of an asset
type AssetDiff struct {
	Hash       string         `json:"hash"`
	Bytes      int64          `json:"bytes"`
	State      AssetDiffState `json:"state"`
	IsUploaded bool           `json:"isUploaded"`
}

// AssetUploadData represents the response from starting an asset upload
type AssetUploadData struct {
	Hash                 string       `json:"hash"`
	Variant              string       `json:"variant,omitempty"`
	Id                   string       `json:"id"`
	OwnerId              string       `json:"ownerId"`
	TotalBytes           int64        `json:"totalBytes"`
	ChunkSize            int64        `json:"chunkSize"`
	TotalChunks          int          `json:"totalChunks"`
	UploadState          string       `json:"uploadState"` // "UploadingChunks", "Finalizing", "Uploaded", "Failed"
	UploadKey            string       `json:"uploadKey"`
	UploadEndpoint       string       `json:"uploadEndpoint"`
	IsDirectUpload       bool         `json:"isDirectUpload"`
	MaxUploadConcurrency int          `json:"maxUploadConcurrency"`
	Chunks               []AssetChunk `json:"chunks,omitempty"`
	CreatedOn            string       `json:"createdOn"`
	LastChunkSize        int64        `json:"lastChunkSize,omitempty"`
}

// AssetChunk represents a chunk of an uploaded asset
type AssetChunk struct {
	Index int    `json:"index"`
	Key   string `json:"key"`
}

// machineID is generated once per process and reused for all record versioning
var machineID = "M-" + uuid.New().String()

// GenerateRecordID generates a new record ID
func GenerateRecordID() string {
	return "R-" + uuid.New().String()
}

// GetMachineID returns the machine ID for record versioning (same value for the lifetime of the process)
func GetMachineID() string {
	return machineID
}

// ComputeAssetHash computes the SHA256 hash of data
func ComputeAssetHash(data []byte) string {
	hashBytes := sha256.Sum256(data)
	return hex.EncodeToString(hashBytes[:])
}

// PreprocessRecord starts preprocessing a record to determine which assets need uploading
func (s *UserSession) PreprocessRecord(ctx context.Context, record *Record) (*RecordPreprocessStatus, error) {
	ownerType := "users"
	if len(record.OwnerId) > 0 && record.OwnerId[0] == 'G' {
		ownerType = "groups"
	}

	reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, record.OwnerId, "records", record.Id, "preprocess")
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}

	body, err := json.Marshal(record)
	if err != nil {
		return nil, errors.Errorf("failed to marshal record: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodPost, reqUrl, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("failed to preprocess record: %s - %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var status RecordPreprocessStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, errors.Errorf("failed to parse preprocess status: %w", err)
	}

	return &status, nil
}

// GetPreprocessStatus gets the current status of record preprocessing
func (s *UserSession) GetPreprocessStatus(ctx context.Context, ownerId, recordId, preprocessId string) (*RecordPreprocessStatus, error) {
	ownerType := "users"
	if len(ownerId) > 0 && ownerId[0] == 'G' {
		ownerType = "groups"
	}

	reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, ownerId, "records", recordId, "preprocess", preprocessId)
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("failed to get preprocess status: %s - %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var status RecordPreprocessStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, errors.Errorf("failed to parse preprocess status: %w", err)
	}

	return &status, nil
}

// WaitForPreprocess waits for preprocessing to complete and returns asset diffs
func (s *UserSession) WaitForPreprocess(ctx context.Context, ownerId, recordId, preprocessId string) ([]AssetDiff, error) {
	for i := 0; i < 60; i++ {
		status, err := s.GetPreprocessStatus(ctx, ownerId, recordId, preprocessId)
		if err != nil {
			return nil, err
		}

		switch status.State {
		case "Success":
			return status.ResultDiffs, nil
		case "Failed":
			return nil, errors.Errorf("preprocess failed: %s", status.FailReason)
		default:
			// Still preprocessing, wait and retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
	}

	return nil, errors.Errorf("preprocess timeout")
}

// StartAssetUpload initiates an asset upload
func (s *UserSession) StartAssetUpload(ctx context.Context, ownerId, hash string, totalBytes int64) (*AssetUploadData, error) {
	ownerType := "users"
	if len(ownerId) > 0 && ownerId[0] == 'G' {
		ownerType = "groups"
	}

	reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, ownerId, "assets", hash, "upload")
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}
	reqUrl = fmt.Sprintf("%s?size=%d", reqUrl, totalBytes)

	req, err := s.makeApiRequest(ctx, http.MethodPost, reqUrl, nil)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	// 400 with "AlreadyUploaded" means asset already exists
	if resp.StatusCode == http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		if bytes.Contains(respBody, []byte("AlreadyUploaded")) {
			return nil, nil // Already uploaded, skip
		}
		return nil, errors.Errorf("failed to start asset upload: %s - %s", resp.Status, string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.Errorf("failed to start asset upload: %s - %s", resp.Status, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	var uploadData AssetUploadData
	if err := json.Unmarshal(respBody, &uploadData); err != nil {
		return nil, errors.Errorf("failed to parse upload data: %w", err)
	}

	return &uploadData, nil
}

// UploadAssetChunks uploads asset data in chunks
func (s *UserSession) UploadAssetChunks(ctx context.Context, uploadData *AssetUploadData, data []byte) ([]AssetChunk, error) {
	var chunks []AssetChunk
	offset := int64(0)

	for i := 0; i < uploadData.TotalChunks; i++ {
		chunkSize := uploadData.ChunkSize
		if i == uploadData.TotalChunks-1 && uploadData.LastChunkSize > 0 {
			chunkSize = uploadData.LastChunkSize
		}

		end := offset + chunkSize
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		chunkData := data[offset:end]
		offset = end

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadData.UploadEndpoint, bytes.NewReader(chunkData))
		if err != nil {
			return nil, errors.Errorf("failed to create chunk request: %w", err)
		}

		req.Header.Set("Upload-Key", uploadData.UploadKey)
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(chunkData)))

		if uploadData.IsDirectUpload {
			req.Header.Set("Upload-Timestamp", uploadData.CreatedOn)
		} else {
			req.Header.Set("Part-Number", fmt.Sprintf("%d", i+1))
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, errors.Errorf("failed to upload chunk %d: %w", i, err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, errors.Errorf("failed to upload chunk %d: %s - %s", i, resp.Status, string(respBody))
		}

		if readErr != nil {
			return nil, errors.Errorf("failed to read chunk %d response: %w", i, readErr)
		}

		if !uploadData.IsDirectUpload {
			// For multipart uploads, collect ETags
			etag := resp.Header.Get("ETag")
			if etag == "" {
				var etagResp struct {
					ETag string `json:"ETag"`
				}
				if err := json.Unmarshal(respBody, &etagResp); err == nil {
					etag = etagResp.ETag
				}
			}
			chunks = append(chunks, AssetChunk{Index: i, Key: etag})
		}
	}

	return chunks, nil
}

// FinalizeAssetUpload finalizes a multipart asset upload
func (s *UserSession) FinalizeAssetUpload(ctx context.Context, ownerId string, uploadData *AssetUploadData, chunks []AssetChunk) error {
	ownerType := "users"
	if len(ownerId) > 0 && ownerId[0] == 'G' {
		ownerType = "groups"
	}

	reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, ownerId, "assets", uploadData.Hash, "upload", uploadData.Id)
	if err != nil {
		return errors.Errorf("failed to make request URL: %w", err)
	}

	uploadData.Chunks = chunks
	body, err := json.Marshal(uploadData)
	if err != nil {
		return errors.Errorf("failed to marshal upload data: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodPatch, reqUrl, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to finalize upload: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// WaitForAssetUpload waits for an asset upload to complete
func (s *UserSession) WaitForAssetUpload(ctx context.Context, ownerId, hash, uploadId string) error {
	ownerType := "users"
	if len(ownerId) > 0 && ownerId[0] == 'G' {
		ownerType = "groups"
	}

	for i := 0; i < 120; i++ {
		reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, ownerId, "assets", hash, "upload", uploadId)
		if err != nil {
			return errors.Errorf("failed to make request URL: %w", err)
		}

		req, err := s.makeApiRequest(ctx, http.MethodGet, reqUrl, nil)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return errors.Wrap(err, 0)
		}

		if resp.StatusCode != http.StatusOK {
			return errors.Errorf("failed to get upload status: %s - %s", resp.Status, string(respBody))
		}

		var status AssetUploadData
		if err := json.Unmarshal(respBody, &status); err != nil {
			return errors.Errorf("failed to parse upload status: %w", err)
		}

		switch status.UploadState {
		case "Uploaded":
			return nil
		case "Failed":
			return errors.Errorf("upload failed")
		default:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
	}

	return errors.Errorf("upload timeout")
}

// UploadAsset uploads a single asset and returns the resdb:// URL
func (s *UserSession) UploadAsset(ctx context.Context, ownerId string, data []byte) (string, error) {
	hash := ComputeAssetHash(data)
	totalBytes := int64(len(data))

	// Start upload
	uploadData, err := s.StartAssetUpload(ctx, ownerId, hash, totalBytes)
	if err != nil {
		return "", err
	}

	// If nil, asset already exists
	if uploadData == nil {
		return fmt.Sprintf("resdb:///%s", hash), nil
	}

	// Upload chunks
	chunks, err := s.UploadAssetChunks(ctx, uploadData, data)
	if err != nil {
		return "", err
	}

	// Finalize if multipart
	if !uploadData.IsDirectUpload {
		if err := s.FinalizeAssetUpload(ctx, ownerId, uploadData, chunks); err != nil {
			return "", err
		}
	}

	// Wait for processing
	if err := s.WaitForAssetUpload(ctx, ownerId, hash, uploadData.Id); err != nil {
		return "", err
	}

	return fmt.Sprintf("resdb:///%s", hash), nil
}

// UpsertRecord creates or updates a record
func (s *UserSession) UpsertRecord(ctx context.Context, record *Record) error {
	ownerType := "users"
	if len(record.OwnerId) > 0 && record.OwnerId[0] == 'G' {
		ownerType = "groups"
	}

	reqUrl, err := url.JoinPath(API_BASE_URL, ownerType, record.OwnerId, "records", record.Id)
	if err != nil {
		return errors.Errorf("failed to make request URL: %w", err)
	}
	reqUrl = reqUrl + "?ensureFolder=true"

	body, err := json.Marshal(record)
	if err != nil {
		return errors.Errorf("failed to marshal record: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodPut, reqUrl, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to upsert record: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// UploadTextureRecord uploads a texture image and creates a record for it
// Returns the record ID and asset URI
func (s *UserSession) UploadTextureRecord(ctx context.Context, name string, path string, imageData []byte) (recordId string, assetUri string, err error) {
	ownerId := s.UserId
	hash := ComputeAssetHash(imageData)
	totalBytes := int64(len(imageData))

	// Create record
	recordId = GenerateRecordID()
	machineId := GetMachineID()
	now := time.Now().UTC().Format(time.RFC3339)

	record := &Record{
		Id:       recordId,
		OwnerId:  ownerId,
		AssetUri: fmt.Sprintf("resdb:///%s", hash),
		Version: RecordVersion{
			GlobalVersion:          0,
			LocalVersion:           1,
			LastModifyingUserId:    ownerId,
			LastModifyingMachineId: machineId,
		},
		Name:                 name,
		RecordType:           "texture",
		Path:                 path,
		LastModificationTime: now,
		IsDeleted:            false,
		IsPublic:             false,
		IsForPatrons:         false,
		IsListed:             false,
		IsReadOnly:           false,
		AssetManifest: []DBAsset{
			{Hash: hash, Bytes: totalBytes},
		},
	}

	// Phase 2: Preprocess record
	preprocessStatus, err := s.PreprocessRecord(ctx, record)
	if err != nil {
		return "", "", errors.Errorf("failed to preprocess record: %w", err)
	}

	// Phase 3: Wait for preprocess
	assetDiffs, err := s.WaitForPreprocess(ctx, ownerId, recordId, preprocessStatus.Id)
	if err != nil {
		return "", "", errors.Errorf("failed to wait for preprocess: %w", err)
	}

	// Phase 4: Upload assets that need uploading
	for _, diff := range assetDiffs {
		if !diff.IsUploaded && diff.Hash == hash {
			// Start upload
			uploadData, err := s.StartAssetUpload(ctx, ownerId, hash, totalBytes)
			if err != nil {
				return "", "", errors.Errorf("failed to start asset upload: %w", err)
			}

			if uploadData != nil {
				// Upload chunks
				chunks, err := s.UploadAssetChunks(ctx, uploadData, imageData)
				if err != nil {
					return "", "", errors.Errorf("failed to upload asset chunks: %w", err)
				}

				// Finalize if multipart
				if !uploadData.IsDirectUpload {
					if err := s.FinalizeAssetUpload(ctx, ownerId, uploadData, chunks); err != nil {
						return "", "", errors.Errorf("failed to finalize upload: %w", err)
					}
				}

				// Wait for processing
				if err := s.WaitForAssetUpload(ctx, ownerId, hash, uploadData.Id); err != nil {
					return "", "", errors.Errorf("failed to wait for upload: %w", err)
				}
			}
		}
	}

	// Phase 6: Upsert record
	if err := s.UpsertRecord(ctx, record); err != nil {
		return "", "", errors.Errorf("failed to upsert record: %w", err)
	}

	return recordId, record.AssetUri, nil
}
