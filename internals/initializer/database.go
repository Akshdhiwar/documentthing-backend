package initializer

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func ConnectToDB() {
	var dbUser string

	// if os.Getenv("RAILS_ENVIRONMENT") == "LOCAL" {
	dbUser = os.Getenv("RAILS_DATABASE_USER")
	// } else {
	// dbUser = os.Getenv("RAILS_DATABASE_USER_PROD")
	// }

	var dbPassword string

	// if os.Getenv("RAILS_ENVIRONMENT") == "LOCAL" {
	dbPassword = os.Getenv("RAILS_DATABASE_PASSWORD")
	// } else {
	// dbPassword = os.Getenv("RAILS_DATABASE_PASSWORD_PROD")
	// }
	dbName := os.Getenv("RAILS_DATABASE_NAME")
	dbHost := os.Getenv("RAILS_DATABASE_HOST")
	dbPort := os.Getenv("RAILS_DATABASE_PORT")

	// Construct DSN
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		dbUser, dbPassword, dbName, dbHost, dbPort)

	if dsn == "" {
		log.Fatal("RAILS_DB environment variable is empty")
	}

	var err error
	// Parse and configure the connection pool settings
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse connection string: %v\n", err)
		os.Exit(1)
	}

	// Optionally set the query execution mode to avoid statement caching issues
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Create a new connection pool with the configured settings
	DB, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
}
