package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct-horse") {
		t.Error("CheckPassword should accept the correct password")
	}
	if CheckPassword(hash, "wrong-battery") {
		t.Error("CheckPassword should reject an incorrect password")
	}
}

func TestDummyHashNeverMatches(t *testing.T) {
	h := DummyHash()
	if h == "" {
		t.Fatal("DummyHash returned empty string")
	}
	for _, pw := range []string{"", "password", "admin", "hunter2", "correct-horse-battery-staple"} {
		if CheckPassword(h, pw) {
			t.Errorf("DummyHash matched password %q", pw)
		}
	}
}
