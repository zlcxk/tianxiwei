package auth

import (
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("CorrectHorseBatteryStaple!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=") {
		t.Fatalf("unexpected hash format: %q", hash)
	}
	if !VerifyPassword(hash, "CorrectHorseBatteryStaple!") {
		t.Fatal("valid password should verify")
	}
	if VerifyPassword(hash, "CorrectHorseBatteryStaple") {
		t.Fatal("wrong password must not verify")
	}
	if VerifyPassword(hash, "") {
		t.Fatal("empty password must not verify")
	}
}

func TestHashPasswordUsesUniqueSalt(t *testing.T) {
	a, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("same-password")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	cases := []string{
		"",
		"plaintext",
		"$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=0,t=3,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=0,p=2$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=0$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=2$***$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$***",
	}
	for _, hash := range cases {
		if VerifyPassword(hash, "anything") {
			t.Fatalf("malformed hash must not verify: %q", hash)
		}
	}
}

func TestSessionAndCSRFTokens(t *testing.T) {
	session, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	csrf, err := NewCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(session) < 40 || len(csrf) < 40 {
		t.Fatalf("tokens look too short: %d, %d", len(session), len(csrf))
	}
	if session == csrf {
		t.Fatal("session and CSRF tokens must differ")
	}
	if TokenHash(session) != TokenHash(session) {
		t.Fatal("token hash must be deterministic")
	}
	if TokenHash(session) == TokenHash(csrf) {
		t.Fatal("token hashes must differ")
	}
}
