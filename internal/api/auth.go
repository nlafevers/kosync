package api

import (
	"crypto/md5"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash of the password using cost 12.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

// KOReaderKey returns the lowercase MD5 hex digest that the KOReader sync
// client transmits in place of the raw password (as the "password" field on
// registration and the X-AUTH-KEY header on every authenticated request).
//
// The server stores bcrypt(md5(password)): the registration handler receives
// the MD5 already computed by the client, so a password entered locally via the
// CLI must be passed through KOReaderKey before HashPassword. Otherwise the CLI
// would store bcrypt(rawpassword), which can never match the md5(password) key
// the client sends at login. This MD5 pre-hash is a KOReader protocol quirk and
// is the one deliberate divergence from the KOPDS CLI, which authenticates raw
// passwords over HTTP Basic Auth and must not apply it.
func KOReaderKey(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}

// CheckPassword compares a bcrypt hashed password with its possible plaintext equivalent.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
