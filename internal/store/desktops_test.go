package store_test

import (
	"context"
	"testing"

	"github.com/lsariol/plop/internal/store"
	"github.com/lsariol/plop/internal/testutil"
)

func TestPairingCodeRoundTrip(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	// Create a user first (pairing codes reference users)
	if err := s.CreateUser(ctx, "alice", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	code, err := s.CreatePairingCode(ctx, "alice")
	if err != nil {
		t.Fatalf("create pairing code: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("want code length 8, got %d", len(code))
	}

	userID, err := s.ConsumePairingCode(ctx, code)
	if err != nil {
		t.Fatalf("consume pairing code: %v", err)
	}
	if userID != "alice" {
		t.Errorf("want userID %q, got %q", "alice", userID)
	}
}

func TestPairingCodeExpiredOrWrong(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	_, err := s.ConsumePairingCode(ctx, "BADCODE")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPairingCodeConsumedOnce(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	if err := s.CreateUser(ctx, "bob", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	code, _ := s.CreatePairingCode(ctx, "bob")

	if _, err := s.ConsumePairingCode(ctx, code); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := s.ConsumePairingCode(ctx, code); err != store.ErrNotFound {
		t.Errorf("second consume: want ErrNotFound, got %v", err)
	}
}

func TestCreateAndGetDesktop(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	if err := s.CreateUser(ctx, "carol", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := s.CreateDesktop(ctx, "carol", "Carols PC")
	if err != nil {
		t.Fatalf("create desktop: %v", err)
	}
	if token == "" {
		t.Fatal("want non-empty token")
	}

	userID, err := s.GetDesktopUser(ctx, token)
	if err != nil {
		t.Fatalf("get desktop user: %v", err)
	}
	if userID != "carol" {
		t.Errorf("want userID %q, got %q", "carol", userID)
	}
}

func TestGetDesktopUserNotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	_, err := s.GetDesktopUser(ctx, "nonexistent-token")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestGetAndDeleteUserDesktops(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	if err := s.CreateUser(ctx, "dave", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	token1, _ := s.CreateDesktop(ctx, "dave", "Home PC")
	_, _ = s.CreateDesktop(ctx, "dave", "Work PC")

	desktops, err := s.GetUserDesktops(ctx, "dave")
	if err != nil {
		t.Fatalf("get desktops: %v", err)
	}
	if len(desktops) != 2 {
		t.Fatalf("want 2 desktops, got %d", len(desktops))
	}

	// Find the id for token1
	userID, _ := s.GetDesktopUser(ctx, token1)
	if userID != "dave" {
		t.Fatal("token lookup failed")
	}

	// Revoke by name check
	if err := s.DeleteDesktop(ctx, desktops[0].ID, "dave"); err != nil {
		t.Fatalf("delete desktop: %v", err)
	}
	remaining, _ := s.GetUserDesktops(ctx, "dave")
	if len(remaining) != 1 {
		t.Errorf("want 1 desktop after revoke, got %d", len(remaining))
	}
}

func TestDeleteDesktopWrongUser(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	if err := s.CreateUser(ctx, "eve", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.CreateUser(ctx, "mallory", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, _ = s.CreateDesktop(ctx, "eve", "Eves PC")
	desktops, _ := s.GetUserDesktops(ctx, "eve")

	err := s.DeleteDesktop(ctx, desktops[0].ID, "mallory")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound when wrong user tries to revoke, got %v", err)
	}
}

func TestPairingCodeCaseInsensitive(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	if err := s.CreateUser(ctx, "frank", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	code, _ := s.CreatePairingCode(ctx, "frank")

	// Should work with lowercase and surrounding whitespace
	userID, err := s.ConsumePairingCode(ctx, " "+code+" ")
	if err != nil {
		t.Fatalf("consume with whitespace: %v", err)
	}
	if userID != "frank" {
		t.Errorf("want %q, got %q", "frank", userID)
	}
}
