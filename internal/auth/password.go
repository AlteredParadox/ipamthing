// ipamthing — a lightweight single-binary IPAM.
// Copyright (C) 2026 AlteredParadox
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE. See the GNU Affero General Public License
// for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
