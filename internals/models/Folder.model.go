package models

import "github.com/google/uuid"

type Folder struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	FileID   uuid.UUID `json:"fileId"`
	Children []Folder  `json:"children"`
}
