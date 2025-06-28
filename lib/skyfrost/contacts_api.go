package skyfrost

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/go-errors/errors"
)

type EntityId struct {
	Id      string `json:"id"`
	OwnerId string `json:"ownerId"`
}

type UserProfile struct {
	IconUrl       string     `json:"iconUrl"`
	Tagline       string     `json:"tagline"`
	DisplayBadges []EntityId `json:"displayBadges"`
	Description   string     `json:"description"`
}

type Contact struct {
	Id         string      `json:"id"`
	Username   string      `json:"contactUsername"`
	Status     string      `json:"contactStatus"`
	IsAccepted bool        `json:"isAccepted"`
	Profile    UserProfile `json:"profile"`
}

func (s *UserSession) GetContacts(ctx context.Context) ([]Contact, error) {
	reqUrl, err := url.JoinPath(API_BASE_URL, "users", s.UserId, "contacts")
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
		return nil, errors.Errorf("failed to get contacts: %s", resp.Status)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	var contacts []Contact
	if err := json.Unmarshal(respBody, &contacts); err != nil {
		return nil, errors.Errorf("failed to decode contacts: %w", err)
	}

	return contacts, nil
}
