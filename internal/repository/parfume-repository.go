package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Product struct {
	Id          string    `json:"Id" db:"id"`
	NameParfume string    `json:"NameParfume" db:"name_parfume"`
	Sex         string    `json:"Sex" db:"sex"`
	Description string    `json:"Description" db:"description"`
	Price       int       `json:"Price" db:"price"`
	PhotoPath   string    `json:"PhotoPath" db:"photo_path"`
	CreatedAt   time.Time `json:"CreatedAt" db:"created_at"`
	UpdatedAt   time.Time `json:"UpdatedAt" db:"updated_at"`
}

type ParfumeRepository struct {
	db *sql.DB
}

func NewParfumeRepository(db *sql.DB) *ParfumeRepository {
	return &ParfumeRepository{
		db: db,
	}
}

// Create a new perfume
func (r *ParfumeRepository) Create(product *Product) error {
	product.Id = uuid.New().String()

	query := `
		INSERT INTO parfume (id, name_parfume, sex, description, price, photo_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	_, err := r.db.Exec(query, product.Id, product.NameParfume, product.Sex, product.Description, product.Price, product.PhotoPath)
	if err != nil {
		return fmt.Errorf("error creating perfume: %w", err)
	}
	return nil
}

// Get all perfumes
func (r *ParfumeRepository) GetAll() ([]Product, error) {
	query := `
		SELECT id, name_parfume, sex, description, price, photo_path, created_at, updated_at
		FROM parfume
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying perfumes: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.Id,
			&product.NameParfume,
			&product.Sex,
			&product.Description,
			&product.Price,
			&product.PhotoPath,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning perfume: %w", err)
		}
		products = append(products, product)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating perfume rows: %w", err)
	}

	return products, nil
}

// Get perfume by ID
func (r *ParfumeRepository) GetByID(id string) (*Product, error) {
	query := `
		SELECT id, name_parfume, sex, description, price, photo_path, created_at, updated_at
		FROM parfume
		WHERE id = ?
	`

	var product Product
	err := r.db.QueryRow(query, id).Scan(
		&product.Id,
		&product.NameParfume,
		&product.Sex,
		&product.Description,
		&product.Price,
		&product.PhotoPath,
		&product.CreatedAt,
		&product.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("perfume not found")
		}
		return nil, fmt.Errorf("error getting perfume: %w", err)
	}

	return &product, nil
}

// Update perfume
func (r *ParfumeRepository) Update(product *Product) error {
	query := `
		UPDATE parfume
		SET name_parfume = ?, sex = ?, description = ?, price = ?, photo_path = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.Exec(query, product.NameParfume, product.Sex, product.Description, product.Price, product.PhotoPath, product.Id)
	if err != nil {
		return fmt.Errorf("error updating perfume: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("perfume not found")
	}

	return nil
}

// Delete perfume
func (r *ParfumeRepository) Delete(id string) error {
	query := `DELETE FROM parfume WHERE id = ?`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting perfume: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("perfume not found")
	}

	return nil
}

// Get perfumes by sex
func (r *ParfumeRepository) GetBySex(sex string) ([]Product, error) {
	query := `
		SELECT id, name_parfume, sex, description, price, photo_path, created_at, updated_at
		FROM parfume
		WHERE sex = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, sex)
	if err != nil {
		return nil, fmt.Errorf("error querying perfumes by sex: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.Id,
			&product.NameParfume,
			&product.Sex,
			&product.Description,
			&product.Price,
			&product.PhotoPath,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning perfume: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// Search perfumes by name or description
func (r *ParfumeRepository) SearchByName(name string) ([]Product, error) {
	query := `
		SELECT id, name_parfume, sex, description, price, photo_path, created_at, updated_at
		FROM parfume
		WHERE name_parfume LIKE ? OR description LIKE ?
		ORDER BY created_at DESC
	`

	searchTerm := "%" + name + "%"
	rows, err := r.db.Query(query, searchTerm, searchTerm)
	if err != nil {
		return nil, fmt.Errorf("error searching perfumes: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.Id,
			&product.NameParfume,
			&product.Sex,
			&product.Description,
			&product.Price,
			&product.PhotoPath,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning perfume: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}

// Advanced search with multiple criteria
func (r *ParfumeRepository) AdvancedSearch(name, sex string, minPrice, maxPrice int) ([]Product, error) {
	query := `
		SELECT id, name_parfume, sex, description, price, photo_path, created_at, updated_at
		FROM parfume
		WHERE 1=1
	`
	var args []interface{}

	if name != "" {
		query += " AND name_parfume LIKE ?"
		args = append(args, "%"+name+"%")
	}

	if sex != "" {
		query += " AND sex = ?"
		args = append(args, sex)
	}

	if minPrice > 0 {
		query += " AND price >= ?"
		args = append(args, minPrice)
	}

	if maxPrice > 0 {
		query += " AND price <= ?"
		args = append(args, maxPrice)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error in advanced search: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&product.Id,
			&product.NameParfume,
			&product.Sex,
			&product.Description,
			&product.Price,
			&product.PhotoPath,
			&product.CreatedAt,
			&product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning perfume: %w", err)
		}
		products = append(products, product)
	}

	return products, nil
}
