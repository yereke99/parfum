package database

import (
	"database/sql"
	"fmt"
	"log"
)

func CreateTables(db *sql.DB) error {
	tables := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		{"just", createJustTable},
		{"parfumes", createParfumesTable},
	}

	for _, table := range tables {
		log.Printf("Creating table: %s", table.name)
		if err := table.fn(db); err != nil {
			return fmt.Errorf("create %s table: %w", table.name, err)
		}
	}

	log.Println("All tables, indexes and views created successfully")
	return nil
}

func createJustTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS just (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL UNIQUE,
		userName VARCHAR(255) NOT NULL,
		dataRegistred VARCHAR(50) NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(stmt)
	return err
}

// Fixed createParfumesTable function - SQLite compatible
func createParfumesTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS parfumes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name_parfume VARCHAR(255) NOT NULL,
		price DECIMAL(10,2) NOT NULL,
		sex TEXT NOT NULL CHECK(sex IN ('Male', 'Female', 'Unisex')),
		description TEXT NOT NULL,
		photo_path VARCHAR(255),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_parfumes_sex ON parfumes(sex);
	CREATE INDEX IF NOT EXISTS idx_parfumes_price ON parfumes(price);
	CREATE INDEX IF NOT EXISTS idx_parfumes_name ON parfumes(name_parfume);
	`
	_, err := db.Exec(stmt)
	return err
}

// Parfume struct for your data model
type Parfume struct {
	Id          int     `json:"Id" db:"id"`
	NameParfume string  `json:"NameParfume" db:"name_parfume"`
	Price       float64 `json:"Price" db:"price"`
	Sex         string  `json:"Sex" db:"sex"`
	Description string  `json:"Description" db:"description"`
	PhotoPath   string  `json:"PhotoPath" db:"photo_path"`
	CreatedAt   string  `json:"CreatedAt" db:"created_at"`
	UpdatedAt   string  `json:"UpdatedAt" db:"updated_at"`
}
