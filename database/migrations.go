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
		owner UUID NOT NULL,
		org TEXT DEFAULT NULL,
		repo_owner TEXT DEFAULT NULL,
		is_published BOOLEAN DEFAULT FALSE
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
		twitter TEXT,
		google_id TEXT,
		token TEXT,
		type TEXT
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	// Create the invite table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS invite (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		invited_at TIMESTAMPTZ DEFAULT now(),
		deleted_at TIMESTAMPTZ,
		user_name TEXT NOT NULL,
		email TEXT NOT NULL,
		project_id UUID NOT NULL,
		is_accepted BOOLEAN DEFAULT FALSE,
		is_revoked BOOLEAN DEFAULT FALSE,
		role TEXT NOT NULL,
		invited_by TEXT NOT NULL,
		project_name TEXT NOT NULL
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	// Create a Organization Table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS organizations (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        created_at TIMESTAMPTZ DEFAULT now(),
        updated_at TIMESTAMPTZ DEFAULT now(),
        deleted_at TIMESTAMPTZ,
        name TEXT NOT NULL,
        owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        email TEXT DEFAULT NULL
		)`)
	// subscription_id TEXT DEFAULT NULL,
	// status BOOLEAN DEFAULT FALSE,
	// max_user INT DEFAULT NULL,
	// subs_name TEXT DEFAULT NULL,
	// plan_id TEXT DEFAULT NULL,
	// user_count INT DEFAULT NULL

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
		role TEXT NOT NULL,
		CONSTRAINT user_project_unique UNIQUE (user_id, project_id)
	)`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	// Create a org-project-user-mapping table
	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS org_project_user_mapping (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        created_at TIMESTAMPTZ DEFAULT now(),
        updated_at TIMESTAMPTZ DEFAULT now(),
        deleted_at TIMESTAMPTZ,
        org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
        project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        role TEXT NOT NULL
    )`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	_, err = initializer.DB.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS org_user_mapping (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        created_at TIMESTAMPTZ DEFAULT now(),
        updated_at TIMESTAMPTZ DEFAULT now(),
        org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
    )`)

	if err != nil {
		log.Fatalf("Failed to execute migration: %v", err)
	}

	log.Println("All migrations executed successfully")

}
