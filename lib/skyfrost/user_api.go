package skyfrost

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
)

type UserInfo struct {
	ID                 string
	UserName           string
	NormalizedUserName string
	IconUrl            string
}

func FetchUserInfo(ctx context.Context, resoniteID string) (*UserInfo, error) {
	url := fmt.Sprintf("https://api.resonite.com/users/%s", resoniteID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("failed to fetch user info: %s", body)
	}

	jsonBody := string(body)
	if !gjson.Valid(jsonBody) {
		return nil, fmt.Errorf("failed to fetch user info: invalid json: %s", jsonBody)
	}

	idValue := gjson.Get(jsonBody, "id")
	if !idValue.Exists() {
		return nil, fmt.Errorf("failed to fetch user info: id not found")
	}
	userNameValue := gjson.Get(jsonBody, "username")
	if !userNameValue.Exists() {
		return nil, fmt.Errorf("failed to fetch user info: username not found")
	}
	normalizedUserNameValue := gjson.Get(jsonBody, "normalizedUsername")
	if !normalizedUserNameValue.Exists() {
		return nil, fmt.Errorf("failed to fetch user info: normalizedUsername not found")
	}

	result := UserInfo{
		ID:                 idValue.String(),
		UserName:           userNameValue.String(),
		NormalizedUserName: normalizedUserNameValue.String(),
	}
	iconUrlValue := gjson.Get(jsonBody, "profile.iconUrl")
	if iconUrlValue.Exists() {
		result.IconUrl = iconUrlValue.String()
	}

	return &result, nil
}
