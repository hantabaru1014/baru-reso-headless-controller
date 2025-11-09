package skyfrost

import "context"

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
}

// DefaultClient is the default implementation of Client using real API calls
type DefaultClient struct{}

// NewDefaultClient creates a new DefaultClient
func NewDefaultClient() *DefaultClient {
	return &DefaultClient{}
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
	userSession, err := UserLogin(ctx, credential, password)
	if err != nil {
		return nil, err
	}
	return userSession.GetStorage(ctx, ownerId)
}

// GetContacts implements Client.GetContacts
func (c *DefaultClient) GetContacts(ctx context.Context, credential, password string) ([]Contact, error) {
	userSession, err := UserLogin(ctx, credential, password)
	if err != nil {
		return nil, err
	}
	return userSession.GetContacts(ctx)
}
