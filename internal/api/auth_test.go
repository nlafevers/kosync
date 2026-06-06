package api

import (
	"crypto/md5"
	"encoding/hex"
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

func TestKOReaderKey(t *testing.T) {
	// "password" -> md5 digest the KOReader client is documented to send.
	if got := KOReaderKey("password"); got != "5f4dcc3b5aa765d61d8327deb882cf99" {
		t.Errorf("KOReaderKey(\"password\") = %q, want %q", got, "5f4dcc3b5aa765d61d8327deb882cf99")
	}

	sum := md5.Sum([]byte("anything"))
	if got := KOReaderKey("anything"); got != hex.EncodeToString(sum[:]) {
		t.Errorf("KOReaderKey returned %q, not the lowercase md5 hex digest", got)
	}
}

// TestCLIPasswordMatchesClientKey verifies the end-to-end contract: a password
// hashed the way the CLI does it (bcrypt(md5(password))) authenticates against
// the md5(password) key the KOReader client sends, while the raw password does
// not. This is the bug that made CLI-created users unable to log in.
func TestCLIPasswordMatchesClientKey(t *testing.T) {
	password := "hunter2"

	hash, err := HashPassword(KOReaderKey(password))
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	clientKey := KOReaderKey(password) // what the client puts in X-AUTH-KEY
	if !CheckPassword(hash, clientKey) {
		t.Error("CLI-created hash did not authenticate against the client's md5 key")
	}

	if CheckPassword(hash, password) {
		t.Error("CLI-created hash should not authenticate against the raw password")
	}
}
