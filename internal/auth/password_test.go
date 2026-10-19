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
