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
		{"clients", createClientsTable},
		{"orders", createOrdersTable},
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

// createClientsTable creates the clients table
func createClientsTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS clients (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id BIGINT NOT NULL UNIQUE,
		fio VARCHAR(255) NOT NULL,
		contact VARCHAR(100) NOT NULL,
		address TEXT NOT NULL,
		latitude VARCHAR(50),
		longitude VARCHAR(50),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_clients_telegram_id ON clients(telegram_id);
	CREATE INDEX IF NOT EXISTS idx_clients_created_at ON clients(created_at);
	`
	_, err := db.Exec(stmt)
	return err
}

// createOrdersTable creates the orders table
func createOrdersTable(db *sql.DB) error {
	const stmt = `
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		telegram_id BIGINT NOT NULL,
		client_id INTEGER NOT NULL,
		cart_data TEXT NOT NULL,
		total_amount INTEGER NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'paid', 'processing', 'shipped', 'delivered', 'cancelled')),
		payment_link TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE,
		FOREIGN KEY (telegram_id) REFERENCES clients(telegram_id)
	);
	
	CREATE INDEX IF NOT EXISTS idx_orders_telegram_id ON orders(telegram_id);
	CREATE INDEX IF NOT EXISTS idx_orders_client_id ON orders(client_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
	CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
	CREATE INDEX IF NOT EXISTS idx_orders_total_amount ON orders(total_amount);
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
				o.telegram_id,
				c.fio as client_name,
				c.contact as client_contact,
				c.address as delivery_address,
				o.total_amount,
				o.status,
				o.created_at as order_date,
				o.updated_at
			FROM orders o
			JOIN clients c ON o.client_id = c.id
			ORDER BY o.created_at DESC`,
		},
		{
			"daily_stats_view",
			`CREATE VIEW IF NOT EXISTS daily_stats_view AS
			SELECT 
				DATE(created_at) as order_date,
				COUNT(*) as total_orders,
				SUM(total_amount) as total_revenue,
				COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_orders,
				COUNT(CASE WHEN status = 'paid' THEN 1 END) as paid_orders,
				COUNT(CASE WHEN status = 'delivered' THEN 1 END) as delivered_orders
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

	// Clean up old pending orders (older than specified days)
	result, err := db.Exec(`
		DELETE FROM orders 
		WHERE status = 'pending' 
		AND created_at < datetime('now', '-' || ? || ' days')
	`, daysOld)

	if err != nil {
		return fmt.Errorf("cleanup old orders: %w", err)
	}

	affected, _ := result.RowsAffected()
	log.Printf("Cleaned up %d old pending orders", affected)

	return nil
}
