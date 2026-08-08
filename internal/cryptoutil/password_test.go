package cryptoutil

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	params := PasswordParams{
		MemoryKiB:   8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltBytes:   16,
		KeyBytes:    32,
	}
	encoded, err := HashPassword("a sufficiently long password", params)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$") {
		t.Fatalf("HashPassword() = %q, want Argon2id PHC string", encoded)
	}
	valid, err := VerifyPassword(encoded, "a sufficiently long password")
	if err != nil || !valid {
		t.Fatalf("VerifyPassword(correct) = %v, %v", valid, err)
	}
	valid, err = VerifyPassword(encoded, "wrong password")
	if err != nil || valid {
		t.Fatalf("VerifyPassword(wrong) = %v, %v", valid, err)
	}
}

func TestVerifyPasswordRejectsUnsafeEncoding(t *testing.T) {
	tests := []string{
		"not-a-password-hash",
		"$argon2id$v=19$m=1,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=8192,t=1,p=1$bad!$bad!",
	}
	for _, encoded := range tests {
		if valid, err := VerifyPassword(encoded, "password"); err == nil || valid {
			t.Fatalf("VerifyPassword(%q) = %v, %v; want rejection", encoded, valid, err)
		}
	}
}
