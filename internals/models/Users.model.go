package models

import (
	"github.com/google/uuid"
)

type Users struct {
	ID uuid.UUID
	// CreatedAt  pgtype.Timestamptz
	// UpdatedAt  pgtype.Timestamptz
	// DeletedAt  pgtype.Timestamptz `db:"deleted_at"`
	AvatarURL  string
	Company    string
	Email      string
	Twitter    string
	GithubID   int
	GithubName string
	Name       string
}
