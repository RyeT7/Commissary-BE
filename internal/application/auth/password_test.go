package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !verifyPassword("correct horse battery staple", hash) {
		t.Error("verifyPassword rejected the correct password")
	}
	if verifyPassword("wrong password", hash) {
		t.Error("verifyPassword accepted a wrong password")
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, _ := hashPassword("same-password")
	b, _ := hashPassword("same-password")
	if a == b {
		t.Error("two hashes of the same password are identical (missing per-hash salt)")
	}
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "notahash", "pbkdf2_sha256$only$two", "bcrypt$1$x$y"} {
		if verifyPassword("whatever", bad) {
			t.Errorf("verifyPassword accepted malformed hash %q", bad)
		}
	}
}
