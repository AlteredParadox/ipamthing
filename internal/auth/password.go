package auth

import "golang.org/x/crypto/bcrypt"

// dummyHash is a valid bcrypt hash that will never match any real password.
// Used by login flows to equalize bcrypt-comparison timing between
// existing-user-wrong-password and nonexistent-user paths so timing cannot
// leak user existence.
var dummyHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte{0, 1, 2, 3}, bcrypt.DefaultCost)
	if err != nil {
		panic("auth: failed to generate dummy bcrypt hash: " + err.Error())
	}
	dummyHash = h
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// DummyHash returns a bcrypt hash that will never match. Callers use it to
// keep login timing constant when the looked-up user does not exist.
func DummyHash() string {
	return string(dummyHash)
}
