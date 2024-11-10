package models

type UserRole string

const (
	RoleAdmin  UserRole = "Admin"
	RoleEditor UserRole = "Editor"
	RoleViever UserRole = "Viewer"
)
