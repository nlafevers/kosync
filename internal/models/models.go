package models

// User represents a user in the system.
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

// Progress represents the reading progress of a document.
type Progress struct {
	Document   string  `json:"document"`
	Percentage float64 `json:"percentage"`
	Progress   string  `json:"progress"`
	DeviceID   string  `json:"device_id"`
	Device     string  `json:"device"`
	Timestamp  int64   `json:"timestamp"` // Server-side arrival time
}
