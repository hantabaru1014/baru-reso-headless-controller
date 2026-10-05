package skyfrost

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/go-errors/errors"
)

// ErrRegistrationNotConfirmed は登録リクエストは受け付けられたが、処理の完了を確認できなかった.
// Resonite 側ではアカウントが作成済み (または作成中) の可能性がある.
var ErrRegistrationNotConfirmed = errors.New("registration was accepted but its completion could not be confirmed")

// errRegistrationFailed は Resonite 側で登録処理が失敗した (アカウントは作成されない).
var errRegistrationFailed = errors.New("registration failed")

// ValidateRegistration は SkyFrost の RegistrationRequest と同じ条件で登録内容を検証する.
func ValidateRegistration(username, email, password string, dateOfBirth, now time.Time) error {
	const (
		minPasswordLength = 8
		minAge            = 16
		maxAge            = 150
	)

	if strings.TrimSpace(username) == "" {
		return errors.New("username is required")
	}

	// "Name <addr>" 形式も ParseAddress は受け付けるので、アドレス単体であることも確認する.
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return errors.New("invalid email")
	}

	if len([]rune(password)) < minPasswordLength ||
		!strings.ContainsFunc(password, unicode.IsDigit) ||
		!strings.ContainsFunc(password, unicode.IsLower) ||
		!strings.ContainsFunc(password, unicode.IsUpper) {
		return errors.New("password must be at least 8 characters and contain digits, lowercase and uppercase letters")
	}

	if dateOfBirth.After(now.AddDate(-minAge, 0, 0)) {
		return errors.New("must be at least 16 years old")
	}

	if dateOfBirth.Before(now.AddDate(-maxAge, 0, 0)) {
		return errors.New("invalid date of birth")
	}

	return nil
}

// RegisterUser は Resonite アカウントを新規登録し、登録処理の完了を待って Resonite ID を返す.
// 登録はクラウド側でキューイングされるので `GET users/{id}/registration` をポーリングする.
// メール認証は登録完了後にユーザーが別途行う.
// Resonite に登録内容を拒否された場合は *APIError を、受け付け後に完了を確認できなかった場合は
// ErrRegistrationNotConfirmed を返す.
func RegisterUser(ctx context.Context, username, email, password string, dateOfBirth time.Time) (string, error) {
	reqUrl, err := url.JoinPath(API_BASE_URL, "users")
	if err != nil {
		return "", errors.Errorf("failed to make request URL: %w", err)
	}

	reqBody, err := json.Marshal(map[string]any{
		"username":    username,
		"email":       email,
		"dateOfBirth": dateOfBirth.Format(time.RFC3339),
		"password": map[string]any{
			"$type":    "password",
			"password": password,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqUrl, bytes.NewReader(reqBody))
	if err != nil {
		return "", errors.Wrap(err, 0)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Uid", headerUidValue)

	status, err := doRegistrationStatusRequest(req)
	if err != nil {
		return "", errors.Errorf("failed to register: %w", err)
	}

	if status.ID == "" || status.Token == "" {
		return "", errors.Errorf("failed to register: id or token not found in response")
	}

	if err := waitForRegistration(ctx, status.ID, status.Token); err != nil {
		if errors.Is(err, errRegistrationFailed) {
			return "", err
		}

		// ポーリングの APIError は登録内容の拒否ではないので型を露出させない.
		return "", errors.Errorf("%w (%s): %v", ErrRegistrationNotConfirmed, status.ID, err)
	}

	return status.ID, nil
}

// registrationStatus は SkyFrost の RegistrationStatus.
type registrationStatus struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	State string `json:"state"` // Pending / Processing / Done / Failed
	Error string `json:"error"`
}

func waitForRegistration(ctx context.Context, id, token string) error {
	const (
		pollInterval         = 2 * time.Second
		maxStatusCheckErrors = 3
	)

	reqUrl, err := url.JoinPath(API_BASE_URL, "users", id, "registration")
	if err != nil {
		return errors.Errorf("failed to make request URL: %w", err)
	}

	reqUrl += "?" + url.Values{"token": []string{token}}.Encode()

	statusCheckErrors := 0

	for range 60 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
		if err != nil {
			return errors.Wrap(err, 0)
		}

		status, err := doRegistrationStatusRequest(req)
		if err != nil {
			statusCheckErrors++
			if statusCheckErrors >= maxStatusCheckErrors {
				return errors.Errorf("failed to check registration status: %w", err)
			}

			continue
		}

		switch status.State {
		case "Done":
			return nil
		case "Failed":
			return errors.Errorf("%w: %s", errRegistrationFailed, status.Error)
		}
	}

	return errors.Errorf("registration of %s timed out", id)
}

func doRegistrationStatusRequest(req *http.Request) (*registrationStatus, error) {
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	if resp.StatusCode > 299 { //nolint:mnd // HTTP 2xx success range
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	status := &registrationStatus{}
	if err := json.Unmarshal(body, status); err != nil {
		return nil, errors.Errorf("failed to parse response: %w", err)
	}

	return status, nil
}
