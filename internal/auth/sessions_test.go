package auth

import (
	"testing"

	"ipamthing/internal/db"
)

func testDB(t *testing.T) *db.TestDB {
	t.Helper()
	d := db.OpenTest(t)
	// sessions FK references users; insert a test user so Create works.
	if _, err := d.DB.Exec(
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (1, 'testuser', 'hash', 0)`,
	); err != nil {
		t.Fatal("insert test user:", err)
	}
	return d
}

func TestSessionCreateAndGet(t *testing.T) {
	d := testDB(t)
	store := NewSessionStore(d.DB)

	sess, err := store.Create(1)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Token == "" {
		t.Fatal("token should not be empty")
	}
	if sess.UserID != 1 {
		t.Errorf("UserID = %d, want 1", sess.UserID)
	}

	got, err := store.Get(sess.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("Get returned nil for a valid token")
	}
	if got.UserID != 1 {
		t.Errorf("Get().UserID = %d, want 1", got.UserID)
	}
}

func TestSessionGetInvalidToken(t *testing.T) {
	d := testDB(t)
	store := NewSessionStore(d.DB)

	got, err := store.Get("nonexistent-token")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("Get should return nil for a nonexistent token")
	}
}

func TestSessionDelete(t *testing.T) {
	d := testDB(t)
	store := NewSessionStore(d.DB)

	sess, _ := store.Create(1)
	if err := store.Delete(sess.Token); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(sess.Token)
	if got != nil {
		t.Error("Get should return nil after Delete")
	}
}

func TestSessionDeleteOthers(t *testing.T) {
	d := testDB(t)
	store := NewSessionStore(d.DB)

	s1, _ := store.Create(1)
	s2, _ := store.Create(1)
	s3, _ := store.Create(1)

	if err := store.DeleteOthers(1, s2.Token); err != nil {
		t.Fatal(err)
	}

	if got, _ := store.Get(s1.Token); got != nil {
		t.Error("s1 should have been deleted")
	}
	if got, _ := store.Get(s2.Token); got == nil {
		t.Error("s2 (kept token) should still be valid")
	}
	if got, _ := store.Get(s3.Token); got != nil {
		t.Error("s3 should have been deleted")
	}
}
