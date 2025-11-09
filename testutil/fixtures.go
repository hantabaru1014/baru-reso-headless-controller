package testutil

import (
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/google/uuid"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

// CreateTestUser creates a test user in the database
func CreateTestUser(t *testing.T, queries *db.Queries, id, password string) db.User {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	resoniteID := pgtype.Text{String: "U-test-user-" + uuid.New().String(), Valid: true}
	iconURL := pgtype.Text{String: "https://example.com/icon.png", Valid: true}

	err = queries.CreateUser(t.Context(), db.CreateUserParams{
		ID:         id,
		Password:   string(hashedPassword),
		ResoniteID: resoniteID,
		IconUrl:    iconURL,
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	user, err := queries.GetUser(t.Context(), id)
	if err != nil {
		t.Fatalf("failed to get created user: %v", err)
	}

	return user
}

// CreateTestHeadlessAccount creates a test headless account in the database
func CreateTestHeadlessAccount(t *testing.T, queries *db.Queries, resoniteID, credential, password string) db.HeadlessAccount {
	t.Helper()

	displayName := pgtype.Text{String: "Test Headless Account", Valid: true}
	iconURL := pgtype.Text{String: "https://example.com/headless-icon.png", Valid: true}

	err := queries.CreateHeadlessAccount(t.Context(), db.CreateHeadlessAccountParams{
		ResoniteID:      resoniteID,
		Credential:      credential,
		Password:        password,
		LastDisplayName: displayName,
		LastIconUrl:     iconURL,
	})
	if err != nil {
		t.Fatalf("failed to create test headless account: %v", err)
	}

	account, err := queries.GetHeadlessAccount(t.Context(), resoniteID)
	if err != nil {
		t.Fatalf("failed to get created headless account: %v", err)
	}

	return account
}

// CreateTestHeadlessHost creates a test headless host in the database
func CreateTestHeadlessHost(t *testing.T, queries *db.Queries, accountID, name string, status entity.HeadlessHostStatus) db.Host {
	t.Helper()

	id := uniuri.New()
	startupConfig := []byte(`{"tickRate": 60.0}`)
	connectString := "test-container-" + id

	host, err := queries.CreateHost(t.Context(), db.CreateHostParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		AccountID:                      accountID,
		OwnerID:                        pgtype.Text{Valid: false},
		LastStartupConfig:              startupConfig,
		LastStartupConfigSchemaVersion: 1,
		ConnectorType:                  "docker",
		ConnectString:                  connectString,
		AutoUpdatePolicy:               int32(entity.HostAutoUpdatePolicy_UNSPECIFIED),
		Memo:                           pgtype.Text{Valid: false},
		StartedAt: pgtype.Timestamptz{
			Valid: true,
			Time:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create test headless host: %v", err)
	}

	return host
}

// CreateTestSession creates a test session in the database
func CreateTestSession(t *testing.T, queries *db.Queries, hostID, name string, status entity.SessionStatus) db.Session {
	t.Helper()

	id := uniuri.New()
	startupParams := []byte(`{"maxUsers": 8}`)

	session, err := queries.UpsertSession(t.Context(), db.UpsertSessionParams{
		ID:                             id,
		Name:                           name,
		Status:                         int32(status),
		StartedAt:                      pgtype.Timestamptz{Valid: true, Time: time.Now()},
		OwnerID:                        pgtype.Text{Valid: false},
		EndedAt:                        pgtype.Timestamptz{Valid: false},
		HostID:                         hostID,
		StartupParameters:              startupParams,
		StartupParametersSchemaVersion: 1,
		AutoUpgrade:                    false,
		Memo:                           pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}

	return session
}
