package testutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
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
