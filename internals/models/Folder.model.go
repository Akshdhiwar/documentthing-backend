package models

import "github.com/google/uuid"

type Folder struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Children []Folder  `json:"children"`
}
