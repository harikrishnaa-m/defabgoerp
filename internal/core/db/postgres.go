package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Use pgx for speed
)

// Connect returns a raw SQL connection pool
func Connect() *sql.DB {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatal("❌ Failed to open driver:", err)
	}

	// Performance Tuning (Matches your dbstore.go)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		log.Fatal("❌ Database Unreachable:", err)
	}

	log.Println("✅ Database Connected")
	return db
}
