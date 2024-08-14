package database

import (
	"context"
	"log"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
)

func Migrations() {
	var err error
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS projects (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMPTZ DEFAULT now(),
		updated_at TIMESTAMPTZ DEFAULT now(),
		deleted_at TIMESTAMPTZ,
		name TEXT NOT NULL , 
		owner UUID NOT NULL
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

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
