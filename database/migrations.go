package database

import (
	"context"
	"log"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
)

func Migrations() {
	var err error

	// Create the projects table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS projects (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMPTZ DEFAULT now(),
		updated_at TIMESTAMPTZ DEFAULT now(),
		deleted_at TIMESTAMPTZ,
		name TEXT NOT NULL, 
		owner UUID NOT NULL
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	// Create the users table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMPTZ DEFAULT now(),
		updated_at TIMESTAMPTZ DEFAULT now(),
		deleted_at TIMESTAMPTZ,
		avatar_url TEXT,
		company TEXT,
		email TEXT,
		github_id INTEGER,
		github_name TEXT,
		name TEXT,
		twitter TEXT
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	// Create the user_project_mapping table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS user_project_mapping (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMPTZ DEFAULT now(),
		updated_at TIMESTAMPTZ DEFAULT now(),
		deleted_at TIMESTAMPTZ,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
		CONSTRAINT user_project_unique UNIQUE (user_id, project_id)
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}
}
