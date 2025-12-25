package skyfrost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const API_BASE_URL = "https://api.resonite.com"

// Apiリクエスト時にヘッダに付与されるマシン構成のハッシュ。ここではランダムに作ってしまう。
var headerUidValue = HashIDToToken(uuid.New().String(), "")

type UserInfo struct {
	ID                 string
	UserName           string
	NormalizedUserName string
	IconUrl            string
}

func FetchUserInfo(ctx context.Context, resoniteID string) (*UserInfo, error) {
	reqUrl, err := url.JoinPath(API_BASE_URL, "users", resoniteID)
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}
	resp, err := http.Get(reqUrl)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, errors.Errorf("failed to fetch user info: %s", body)
	}

	jsonBody := string(body)
	if !gjson.Valid(jsonBody) {
		return nil, errors.Errorf("failed to fetch user info: invalid json: %s", jsonBody)
	}

	idValue := gjson.Get(jsonBody, "id")
	if !idValue.Exists() {
		return nil, errors.Errorf("failed to fetch user info: id not found")
	}
	userNameValue := gjson.Get(jsonBody, "username")
	if !userNameValue.Exists() {
		return nil, errors.Errorf("failed to fetch user info: username not found")
	}
	normalizedUserNameValue := gjson.Get(jsonBody, "normalizedUsername")
	if !normalizedUserNameValue.Exists() {
		return nil, errors.Errorf("failed to fetch user info: normalizedUsername not found")
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

type UserSession struct {
	UserId string
	token  string
	expire time.Time
}

// IsValid returns true if the session is still valid (not expired)
func (s *UserSession) IsValid() bool {
	// 有効期限の1分前には再ログインするようにする
	return time.Now().Add(time.Minute).Before(s.expire)
}

func UserLogin(ctx context.Context, credential, password string) (*UserSession, error) {
	reqUrl, err := url.JoinPath(API_BASE_URL, "userSessions")
	if err != nil {
		return nil, errors.Errorf("failed to make request URL: %w", err)
	}
	secretMachineId, err := uuid.NewRandom()
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	reqObj := map[string]interface{}{
		"authentication": map[string]interface{}{
			"$type":    "password",
			"password": password,
		},
		"secretMachineId": secretMachineId.String(),
		"rememberMe":      false,
	}
	if strings.HasPrefix(credential, "U-") {
		reqObj["ownerId"] = credential
	} else {
		reqObj["email"] = credential
	}
	reqBody, err := json.Marshal(reqObj)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqUrl, bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("UID", headerUidValue)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Errorf("failed to login: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, errors.Errorf("failed to login: %s", body)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	type RespEntity struct {
		UserId string `json:"userId"`
		Token  string `json:"token"`
		Expire string `json:"expire"`
	}
	type RespJson struct {
		Entity RespEntity `json:"entity"`
	}
	respEntity := &RespJson{}
	err = json.Unmarshal(respBody, respEntity)
	if err != nil {
		return nil, errors.Errorf("failed to parse response: %w", err)
	}

	expireTime, err := time.Parse(time.RFC3339, respEntity.Entity.Expire)
	if err != nil {
		return nil, errors.Errorf("failed to parse expire time: %w", err)
	}

	newSession := &UserSession{
		UserId: respEntity.Entity.UserId,
		token:  respEntity.Entity.Token,
		expire: expireTime,
	}
	return newSession, nil
}

func (s *UserSession) makeApiRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	authToken := fmt.Sprintf("res %s:%s", s.UserId, s.token)
	req.Header.Set("Authorization", authToken)

	return req, nil
}

// UpdateUserProfile updates the user's profile
// PUT users/{userId}/profile
func (s *UserSession) UpdateUserProfile(ctx context.Context, profile *UserProfile) error {
	reqUrl, err := url.JoinPath(API_BASE_URL, "users", s.UserId, "profile")
	if err != nil {
		return errors.Errorf("failed to make request URL: %w", err)
	}

	reqBody, err := json.Marshal(profile)
	if err != nil {
		return errors.Errorf("failed to marshal profile: %w", err)
	}

	req, err := s.makeApiRequest(ctx, http.MethodPut, reqUrl, bytes.NewReader(reqBody))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return errors.Errorf("failed to update profile: %s - %s", resp.Status, string(body))
	}

	return nil
}
