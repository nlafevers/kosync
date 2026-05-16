package api

import (
	"testing"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "mypassword"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(hash, password) {
		t.Error("CheckPassword failed with correct password")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword succeeded with incorrect password")
	}
}
