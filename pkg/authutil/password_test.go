package authutil

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	if hash == "mysecretpassword" {
		t.Fatal("HashPassword returned the original password")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("HashPassword output missing argon2id prefix: %s", hash)
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	password := "correcthorsebatterystaple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword returned false for correct password")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	hash, err := HashPassword("rightpassword")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	ok, err := VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword returned error: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword returned true for wrong password")
	}
}

func TestHashPassword_Deterministic(t *testing.T) {
	password := "samepassword"
	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword (1) returned error: %v", err)
	}
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword (2) returned error: %v", err)
	}
	if hash1 == hash2 {
		t.Fatal("two hashes of the same password should differ (random salt)")
	}
}
