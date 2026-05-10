package main

import "testing"

func TestPasswordHashing(t *testing.T) {
	password := "my-secret-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if hash == password {
		t.Error("hash should not be equal to password")
	}

	if !CheckPassword(password, hash) {
		t.Error("password check should have passed")
	}

	if CheckPassword("wrong-password", hash) {
		t.Error("password check should have failed")
	}
}
