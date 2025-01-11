package models

// Error represents an error response.
type Error struct {
	Code    int    `json:"Code"`              // HTTP status code.
	Message string `json:"Message"`           // Error message.
	Details string `json:"Details,omitempty"` // Additional error details.
}
