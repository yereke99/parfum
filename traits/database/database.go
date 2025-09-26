package database

import (
	"database/sql"
	"fmt"
	"log"
)

// CreateTables creates all required tables for the Lumen application
func CreateTables(db *sql.DB) error {
	tables := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		{"just", createJustTable},
		{"parfumes", createParfumesTable},
		{"client", createClientTable},
		{"loto", createLotoTable},
		{"orders", CreateOrderTable}, // Updated to use new schema
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

// createJustTable creates the just table (existing)
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

// createParfumesTable creates the parfumes table
func createParfumesTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS parfumes (
		id TEXT PRIMARY KEY,
		name_parfume VARCHAR(255) NOT NULL,
		sex VARCHAR(10) NOT NULL CHECK(sex IN ('Male', 'Female', 'Unisex')),
		description TEXT NOT NULL,
		price INTEGER NOT NULL,
		photo_path VARCHAR(500),
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

func createClientTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS client (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL UNIQUE,
		userName VARCHAR(255) NOT NULL,
		fio TEXT NULL,
		contact VARCHAR(50) NOT NULL,
		address TEXT NULL,
		dateRegister VARCHAR(50) NULL,
		dataPay VARCHAR(50) NOT NULL,
		checks BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(stmt)
	return err
}

// CreateOrderTable creates the orders table with the new schema
func CreateOrderTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL,
		userName VARCHAR(255) NOT NULL,
		quantity INT,
		parfumes TEXT NULL,
		fio TEXT NULL,
		contact VARCHAR(50) NOT NULL,
		address TEXT NULL,
		dateRegister VARCHAR(50) NULL,
		dataPay VARCHAR(50) NOT NULL,
		checks BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_orders_id_user ON orders(id_user);
	CREATE INDEX IF NOT EXISTS idx_orders_checks ON orders(checks);
	CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
	`
	_, err := db.Exec(stmt)
	return err
}

// CreateViews creates useful views for reporting
func CreateViews(db *sql.DB) error {
	views := []struct {
		name string
		sql  string
	}{
		{
			"order_summary_view",
			`CREATE VIEW IF NOT EXISTS order_summary_view AS
			SELECT 
				o.id,
				o.id_user,
				o.userName,
				o.fio,
				o.contact,
				o.address,
				o.quantity,
				o.parfumes,
				o.dataPay,
				o.checks,
				o.created_at as order_date,
				o.updated_at
			FROM orders o
			ORDER BY o.created_at DESC`,
		},
		{
			"daily_stats_view",
			`CREATE VIEW IF NOT EXISTS daily_stats_view AS
			SELECT 
				DATE(created_at) as order_date,
				COUNT(*) as total_orders,
				SUM(quantity) as total_quantity,
				COUNT(CASE WHEN checks = 1 THEN 1 END) as checked_orders,
				COUNT(CASE WHEN checks = 0 THEN 1 END) as unchecked_orders
			FROM orders
			GROUP BY DATE(created_at)
			ORDER BY order_date DESC`,
		},
	}

	for _, view := range views {
		log.Printf("Creating view: %s", view.name)
		_, err := db.Exec(view.sql)
		if err != nil {
			return fmt.Errorf("create view %s: %w", view.name, err)
		}
	}

	return nil
}

// SeedData adds sample data for testing (optional)
func SeedData(db *sql.DB) error {
	// Check if data already exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM parfumes").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Sample data already exists, skipping seed")
		return nil
	}

	log.Println("Seeding sample parfume data...")

	samplePerfumes := []struct {
		id          string
		name        string
		sex         string
		description string
		price       int
	}{
		{
			"lumen-001",
			"Lumen Noir",
			"Unisex",
			"Элегантный унисекс аромат с нотами черного перца, амбры и сандалового дерева. Идеально подходит для вечерних мероприятий.",
			25000,
		},
		{
			"lumen-002",
			"Lumen Rose Gold",
			"Female",
			"Женственный аромат с нотами розы, пиона и белого мускуса. Создает ауру изысканности и грации.",
			22000,
		},
		{
			"lumen-003",
			"Lumen Silver",
			"Male",
			"Мужской аромат с нотами бергамота, кедра и ветивера. Символ силы и уверенности.",
			24000,
		},
		{
			"lumen-004",
			"Lumen Crystal",
			"Female",
			"Свежий цветочный аромат с нотами жасмина, лилии и цитрусов. Легкий и воздушный.",
			20000,
		},
		{
			"lumen-005",
			"Lumen Platinum",
			"Male",
			"Премиальный мужской аромат с нотами табака, кожи и ванили. Роскошь в каждой капле.",
			30000,
		},
	}

	for _, perfume := range samplePerfumes {
		_, err := db.Exec(`
			INSERT INTO parfumes (id, name_parfume, sex, description, price)
			VALUES (?, ?, ?, ?, ?)
		`, perfume.id, perfume.name, perfume.sex, perfume.description, perfume.price)

		if err != nil {
			return fmt.Errorf("insert sample perfume %s: %w", perfume.name, err)
		}
	}

	log.Println("Sample data seeded successfully")
	return nil
}

// Update createLotoTable to include checks column
func createLotoTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS loto (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		id_user BIGINT NOT NULL,
		id_loto INT NOT NULL,
		qr TEXT NULL,
		who_paid VARCHAR(255) DEFAULT '',
		receipt TEXT NULL,
		fio TEXT NULL,
		contact VARCHAR(50),
		address TEXT NULL,
		dataPay VARCHAR(50) NOT NULL,
		checks BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(id_user, id_loto)
	);
	`
	_, err := db.Exec(stmt)
	return err
}

// MigrateDatabase performs any necessary migrations
func MigrateDatabase(db *sql.DB) error {
	log.Println("Running database migrations...")

	// Add any future migrations here
	migrations := []struct {
		version string
		sql     string
	}{
		{
			"v1.1.0",
			"ALTER TABLE orders ADD COLUMN delivery_notes TEXT DEFAULT '';",
		},
		{
			"v1.2.0",
			"ALTER TABLE clients ADD COLUMN preferred_language VARCHAR(5) DEFAULT 'kz';",
		},
	}

	for _, migration := range migrations {
		// Simple migration tracking - just try to run and ignore if column exists
		_, err := db.Exec(migration.sql)
		if err != nil {
			// Log but don't fail - likely means migration already ran
			log.Printf("Migration %s: %v (likely already applied)", migration.version, err)
		} else {
			log.Printf("Applied migration %s successfully", migration.version)
		}
	}

	return nil
}

// CleanupOldData removes old data (optional cleanup task)
func CleanupOldData(db *sql.DB, daysOld int) error {
	if daysOld <= 0 {
		return fmt.Errorf("daysOld must be positive")
	}

	log.Printf("Cleaning up data older than %d days...", daysOld)

	// Clean up old unchecked orders (older than specified days)
	result, err := db.Exec(`
		DELETE FROM orders 
		WHERE checks = 0 
		AND created_at < datetime('now', '-' || ? || ' days')
	`, daysOld)

	if err != nil {
		return fmt.Errorf("cleanup old orders: %w", err)
	}

	affected, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old unchecked orders", affected)

	return nil
}
