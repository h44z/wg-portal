package model

type BulkPeerRequest struct {
	Identifiers []string `json:"Identifiers" binding:"required"`
	Reason      string   `json:"Reason"`
}

type BulkUserRequest struct {
	Identifiers []string `json:"Identifiers" binding:"required"`
}
