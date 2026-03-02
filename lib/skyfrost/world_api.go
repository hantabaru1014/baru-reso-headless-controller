package skyfrost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-errors/errors"
)

const maxWorldSearchCount = 20

// WorldRecord represents a world record from the Resonite API.
type WorldRecord struct {
	ID           string
	OwnerID      string
	OwnerName    string
	Name         string
	Description  string
	ThumbnailUri string
	IsFeatured   bool
}

// SearchWorldsResult represents the result of a world search.
type SearchWorldsResult struct {
	Records []WorldRecord
	HasMore bool
}

// searchWorldsRequest is the request body for the records/pagedSearch API.
type searchWorldsRequest struct {
	RecordType    string   `json:"recordType"`
	Count         int      `json:"count"`
	Offset        int      `json:"offset"`
	Private       bool     `json:"private"`
	OnlyFeatured  bool     `json:"onlyFeatured"`
	SortBy        string   `json:"sortBy"`
	SortDirection string   `json:"sortDirection"`
	SubmittedTo   string   `json:"submittedTo,omitempty"`
	ByOwner       string   `json:"byOwner,omitempty"`
	OptionalTags  []string `json:"optionalTags,omitempty"`
	RequiredTags  []string `json:"requiredTags,omitempty"`
	ExcludedTags  []string `json:"excludedTags,omitempty"`
}

// searchWorldsResponse is the response from the records/pagedSearch API.
type searchWorldsResponse struct {
	Records []struct {
		ID          string `json:"id"`
		OwnerID     string `json:"ownerId"`
		OwnerName   string `json:"ownerName"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Submissions []struct {
			Featured bool `json:"featured"`
		} `json:"submissions"`
		ThumbnailUri string `json:"thumbnailUri"`
	} `json:"records"`
}

// parseSearchTerms parses search query into optional, required, and excluded tags
// Based on go.resonite.com search implementation:
// - "+term" -> required tag
// - "-term" -> excluded tag
// - "term" -> optional tag.
func parseSearchTerms(query string) ([]string, []string, []string) {
	var optional, required, excluded []string

	terms := strings.FieldsSeq(strings.TrimSpace(query))
	for term := range terms {
		if len(term) == 0 {
			continue
		}

		switch term[0] {
		case '+':
			if len(term) > 1 {
				required = append(required, term[1:])
			}
		case '-':
			if len(term) > 1 {
				excluded = append(excluded, term[1:])
			}
		default:
			optional = append(optional, term)
		}
	}

	return optional, required, excluded
}

// SearchWorlds searches for published worlds on Resonite.
func SearchWorlds(ctx context.Context, query string, featuredOnly bool, pageIndex int) (*SearchWorldsResult, error) {
	optionalTags, requiredTags, excludedTags := parseSearchTerms(query)

	reqBody := searchWorldsRequest{
		RecordType:    "world",
		Count:         maxWorldSearchCount,
		Offset:        maxWorldSearchCount * pageIndex,
		Private:       false,
		OnlyFeatured:  featuredOnly,
		SortBy:        "FirstPublishTime",
		SortDirection: "Descending",
		SubmittedTo:   "G-Resonite",
		OptionalTags:  optionalTags,
		RequiredTags:  requiredTags,
		ExcludedTags:  excludedTags,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, API_BASE_URL+"/records/pagedSearch", bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, errors.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Errorf("failed to search worlds: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("search worlds failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp searchWorldsResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, errors.Errorf("failed to parse response: %w", err)
	}

	records := make([]WorldRecord, 0, len(searchResp.Records))

	for _, r := range searchResp.Records {
		isFeatured := false
		if len(r.Submissions) > 0 {
			isFeatured = r.Submissions[0].Featured
		}

		records = append(records, WorldRecord{
			ID:           r.ID,
			OwnerID:      r.OwnerID,
			OwnerName:    r.OwnerName,
			Name:         r.Name,
			Description:  r.Description,
			ThumbnailUri: r.ThumbnailUri,
			IsFeatured:   isFeatured,
		})
	}

	return &SearchWorldsResult{
		Records: records,
		HasMore: len(searchResp.Records) >= maxWorldSearchCount,
	}, nil
}

// SearchOwnWorlds searches for worlds owned by the authenticated user.
func (s *UserSession) SearchOwnWorlds(ctx context.Context, pageIndex int) (*SearchWorldsResult, error) {
	reqBody := searchWorldsRequest{
		RecordType:    "world",
		Count:         maxWorldSearchCount,
		Offset:        maxWorldSearchCount * pageIndex,
		Private:       true,
		SortBy:        "LastUpdateDate",
		SortDirection: "Descending",
		ByOwner:       s.UserId,
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.Errorf("failed to marshal request: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodPost, API_BASE_URL+"/records/pagedSearch", bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, errors.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Errorf("failed to search own worlds: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("search own worlds failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp searchWorldsResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, errors.Errorf("failed to parse response: %w", err)
	}

	records := make([]WorldRecord, 0, len(searchResp.Records))

	for _, r := range searchResp.Records {
		isFeatured := false
		if len(r.Submissions) > 0 {
			isFeatured = r.Submissions[0].Featured
		}

		records = append(records, WorldRecord{
			ID:           r.ID,
			OwnerID:      r.OwnerID,
			OwnerName:    r.OwnerName,
			Name:         r.Name,
			Description:  r.Description,
			ThumbnailUri: r.ThumbnailUri,
			IsFeatured:   isFeatured,
		})
	}

	return &SearchWorldsResult{
		Records: records,
		HasMore: len(searchResp.Records) >= maxWorldSearchCount,
	}, nil
}
