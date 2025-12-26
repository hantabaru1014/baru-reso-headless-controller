package skyfrost

import (
	"context"
	"sync"
)

// Client is an interface for Resonite API client operations
type Client interface {
	// UserLogin logs in with the given credentials and returns a user session
	UserLogin(ctx context.Context, credential, password string) (*UserSession, error)
	// FetchUserInfo fetches user information by Resonite ID
	FetchUserInfo(ctx context.Context, resoniteID string) (*UserInfo, error)
	// GetStorageInfo gets storage information for a user
	GetStorageInfo(ctx context.Context, credential, password, ownerId string) (*StorageInfo, error)
	// GetContacts gets contacts for a user by logging in with the given credentials
	GetContacts(ctx context.Context, credential, password string) ([]Contact, error)
	// UploadTextureRecord uploads a texture image and creates a record for it
	// Returns the record ID and asset URI
	UploadTextureRecord(ctx context.Context, credential, password, name, path string, imageData []byte) (recordId string, assetUri string, err error)
	// UpdateUserProfile updates the user's profile
	UpdateUserProfile(ctx context.Context, credential, password string, profile *UserProfile) error
}

// DefaultClient is the default implementation of Client using real API calls
type DefaultClient struct {
	mu       sync.RWMutex
	sessions map[string]*UserSession // key: credential
}

// NewDefaultClient creates a new DefaultClient
func NewDefaultClient() *DefaultClient {
	return &DefaultClient{
		sessions: make(map[string]*UserSession),
	}
}

// getOrLogin returns a cached session if valid, otherwise logs in and caches the new session
func (c *DefaultClient) getOrLogin(ctx context.Context, credential, password string) (*UserSession, error) {
	// まずRead lockでキャッシュを確認
	c.mu.RLock()
	session, exists := c.sessions[credential]
	c.mu.RUnlock()

	if exists && session.IsValid() {
		return session, nil
	}

	// キャッシュが無いか無効なので新しくログイン
	newSession, err := UserLogin(ctx, credential, password)
	if err != nil {
		return nil, err
	}

	// キャッシュに保存
	c.mu.Lock()
	c.sessions[credential] = newSession
	c.mu.Unlock()

	return newSession, nil
}

// UserLogin implements Client.UserLogin
func (c *DefaultClient) UserLogin(ctx context.Context, credential, password string) (*UserSession, error) {
	return UserLogin(ctx, credential, password)
}

// FetchUserInfo implements Client.FetchUserInfo
func (c *DefaultClient) FetchUserInfo(ctx context.Context, resoniteID string) (*UserInfo, error) {
	return FetchUserInfo(ctx, resoniteID)
}

// GetStorageInfo implements Client.GetStorageInfo
func (c *DefaultClient) GetStorageInfo(ctx context.Context, credential, password, ownerId string) (*StorageInfo, error) {
	userSession, err := c.getOrLogin(ctx, credential, password)
	if err != nil {
		return nil, err
	}
	return userSession.GetStorage(ctx, ownerId)
}

// GetContacts implements Client.GetContacts
func (c *DefaultClient) GetContacts(ctx context.Context, credential, password string) ([]Contact, error) {
	userSession, err := c.getOrLogin(ctx, credential, password)
	if err != nil {
		return nil, err
	}
	return userSession.GetContacts(ctx)
}

// UploadTextureRecord implements Client.UploadTextureRecord
func (c *DefaultClient) UploadTextureRecord(ctx context.Context, credential, password, name, path string, imageData []byte) (recordId string, assetUri string, err error) {
	userSession, err := c.getOrLogin(ctx, credential, password)
	if err != nil {
		return "", "", err
	}
	return userSession.UploadTextureRecord(ctx, name, path, imageData)
}

// UpdateUserProfile implements Client.UpdateUserProfile
func (c *DefaultClient) UpdateUserProfile(ctx context.Context, credential, password string, profile *UserProfile) error {
	userSession, err := c.getOrLogin(ctx, credential, password)
	if err != nil {
		return err
	}
	return userSession.UpdateUserProfile(ctx, profile)
}
