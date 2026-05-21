package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsariol/plop/internal/store"
	"github.com/lsariol/plop/internal/testutil"
)

func seedUser(t *testing.T, s *store.Store, username string) {
	t.Helper()
	if err := s.CreateUser(context.Background(), username, "password123"); err != nil {
		t.Fatalf("seed user %q: %v", username, err)
	}
}

func TestSaveAndGetPayload(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())
	seedUser(t, s, "grace")

	p := store.Payload{
		ID:        "test-id-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "grace",
		Text:      "hello world",
		Tags:      []string{"work"},
	}
	if err := s.SavePayload(ctx, p, nil); err != nil {
		t.Fatalf("save payload: %v", err)
	}

	got, err := s.GetPayload(ctx, "test-id-001", "grace")
	if err != nil {
		t.Fatalf("get payload: %v", err)
	}
	if got.Text != "hello world" {
		t.Errorf("want text %q, got %q", "hello world", got.Text)
	}
	if got.UserID != "grace" {
		t.Errorf("want userID %q, got %q", "grace", got.UserID)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "work" {
		t.Errorf("want tags [work], got %v", got.Tags)
	}
}

func TestGetPayloadNotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	_, err := s.GetPayload(ctx, "does-not-exist", "anyone")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestPendingPayloadsFilteredByUser(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())
	seedUser(t, s, "henry")
	seedUser(t, s, "iris")

	for i, owner := range []string{"henry", "henry", "iris"} {
		p := store.Payload{
			ID:        "pending-" + owner + "-" + strings.Repeat("x", i),
			ExpiresAt: time.Now().Add(time.Hour),
			UserID:    owner,
			Tags:      []string{},
		}
		if err := s.SavePayload(ctx, p, nil); err != nil {
			t.Fatalf("save payload: %v", err)
		}
	}

	henryPayloads, err := s.PendingPayloads(ctx, "henry")
	if err != nil {
		t.Fatalf("pending payloads: %v", err)
	}
	if len(henryPayloads) != 2 {
		t.Errorf("want 2 pending for henry, got %d", len(henryPayloads))
	}

	irisPayloads, _ := s.PendingPayloads(ctx, "iris")
	if len(irisPayloads) != 1 {
		t.Errorf("want 1 pending for iris, got %d", len(irisPayloads))
	}
}

func TestAckPayload(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())
	seedUser(t, s, "jake")

	p := store.Payload{
		ID:        "ack-test-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "jake",
		Tags:      []string{},
	}
	if err := s.SavePayload(ctx, p, nil); err != nil {
		t.Fatalf("save payload: %v", err)
	}

	if err := s.AckPayload(ctx, "ack-test-001", "jake"); err != nil {
		t.Fatalf("ack payload: %v", err)
	}

	// Payload should be gone after ack
	_, err := s.GetPayload(ctx, "ack-test-001", "jake")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after ack, got %v", err)
	}
}

func TestAckPayloadNotFound(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())

	err := s.AckPayload(ctx, "nonexistent-id", "anyone")
	if err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestNilTagsCoercedToEmpty(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())
	seedUser(t, s, "kate")

	p := store.Payload{
		ID:        "nil-tags-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "kate",
		Tags:      nil, // should not cause NOT NULL violation
	}
	if err := s.SavePayload(ctx, p, nil); err != nil {
		t.Fatalf("save payload with nil tags: %v", err)
	}
}

// TestCrossUserPayloadDenied verifies that GetPayload and AckPayload reject
// requests from a user who does not own the payload.
func TestCrossUserPayloadDenied(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	s := store.New(pool, t.TempDir())
	seedUser(t, s, "alice")
	seedUser(t, s, "bob")

	p := store.Payload{
		ID:        "cross-user-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "alice",
		Tags:      []string{},
	}
	if err := s.SavePayload(ctx, p, nil); err != nil {
		t.Fatalf("save payload: %v", err)
	}

	// Bob must not be able to read Alice's payload.
	if _, err := s.GetPayload(ctx, "cross-user-001", "bob"); err != store.ErrNotFound {
		t.Errorf("GetPayload: want ErrNotFound for wrong user, got %v", err)
	}

	// Bob must not be able to ack (delete) Alice's payload.
	if err := s.AckPayload(ctx, "cross-user-001", "bob"); err != store.ErrNotFound {
		t.Errorf("AckPayload: want ErrNotFound for wrong user, got %v", err)
	}

	// Alice should still be able to read her own payload.
	if _, err := s.GetPayload(ctx, "cross-user-001", "alice"); err != nil {
		t.Errorf("GetPayload: alice should still own payload, got %v", err)
	}
}

// TestDuplicateFilenameDeduplication verifies that uploading two files with
// the same name results in both being saved with distinct names.
func TestDuplicateFilenameDeduplication(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	s := store.New(pool, dir)
	seedUser(t, s, "luna")

	uploads := []store.UploadedFile{
		{Name: "report.pdf", MimeType: "application/pdf", Reader: strings.NewReader("v1")},
		{Name: "report.pdf", MimeType: "application/pdf", Reader: strings.NewReader("v2")},
	}
	p := store.Payload{
		ID:        "dedup-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "luna",
		Tags:      []string{},
	}
	if err := s.SavePayload(ctx, p, uploads); err != nil {
		t.Fatalf("save payload: %v", err)
	}

	filesDir := filepath.Join(dir, "dedup-001", "files")
	entries, err := os.ReadDir(filesDir)
	if err != nil {
		t.Fatalf("read files dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("want 2 files on disk, got %d", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !names["report.pdf"] {
		t.Error("want report.pdf to exist")
	}
	if !names["report (1).pdf"] {
		t.Error("want report (1).pdf to exist")
	}
}

// TestBackslashPathTraversal verifies that a Windows-style traversal filename
// is sanitized to just the base component on a Linux server.
func TestBackslashPathTraversal(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	dir := t.TempDir()
	s := store.New(pool, dir)
	seedUser(t, s, "mallory")

	uploads := []store.UploadedFile{
		{Name: `..\..\evil.txt`, MimeType: "text/plain", Reader: strings.NewReader("pwned")},
	}
	p := store.Payload{
		ID:        "traversal-001",
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    "mallory",
		Tags:      []string{},
	}
	if err := s.SavePayload(ctx, p, uploads); err != nil {
		t.Fatalf("save payload: %v", err)
	}

	// File must land inside the payload directory, not outside it.
	expected := filepath.Join(dir, "traversal-001", "files", "evil.txt")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("want file at %s, got error: %v", expected, err)
	}

	// The parent dir must not contain any unexpected file.
	outside := filepath.Join(dir, "evil.txt")
	if _, err := os.Stat(outside); err == nil {
		t.Errorf("traversal succeeded: file written to %s", outside)
	}
}
