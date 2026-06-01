package models

// Progress represents the reading progress of a document.
type Progress struct {
	Document   string  `json:"document"`
	Percentage float64 `json:"percentage"`
	Progress   string  `json:"progress"`
	DeviceID   string  `json:"device_id"`
	Device     string  `json:"device"`
	Timestamp  int64   `json:"timestamp"` // Server-side arrival time
}
