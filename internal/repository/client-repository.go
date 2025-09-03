package repository

import (
	"database/sql"
	"parfum/internal/domain"
	"time"
)

type ClientRepository struct {
	db *sql.DB
}

func NewClientRepository(db *sql.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

// SaveOrUpdate creates or updates a client
func (r *ClientRepository) SaveOrUpdate(client *domain.Client) error {
	// Check if client exists
	existingClient, err := r.GetByTelegramID(client.TelegramID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if existingClient != nil {
		// Update existing client
		query := `
			UPDATE clients 
			SET fio = ?, contact = ?, address = ?, latitude = ?, longitude = ?, updated_at = CURRENT_TIMESTAMP 
			WHERE telegram_id = ?
		`
		_, err = r.db.Exec(query, client.FIO, client.Contact, client.Address, client.Latitude, client.Longitude, client.TelegramID)
		if err != nil {
			return err
		}
		client.ID = existingClient.ID
	} else {
		// Create new client
		query := `
			INSERT INTO clients (telegram_id, fio, contact, address, latitude, longitude, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		result, err := r.db.Exec(query, client.TelegramID, client.FIO, client.Contact, client.Address, client.Latitude, client.Longitude)
		if err != nil {
			return err
		}

		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		client.ID = id
	}

	return nil
}

// GetByTelegramID retrieves a client by telegram ID
func (r *ClientRepository) GetByTelegramID(telegramID int64) (*domain.Client, error) {
	query := `
		SELECT id, telegram_id, fio, contact, address, latitude, longitude, created_at, updated_at
		FROM clients 
		WHERE telegram_id = ?
	`

	row := r.db.QueryRow(query, telegramID)

	var client domain.Client
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&client.ID,
		&client.TelegramID,
		&client.FIO,
		&client.Contact,
		&client.Address,
		&client.Latitude,
		&client.Longitude,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	client.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	client.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

	return &client, nil
}

// GetByID retrieves a client by ID
func (r *ClientRepository) GetByID(id int64) (*domain.Client, error) {
	query := `
		SELECT id, telegram_id, fio, contact, address, latitude, longitude, created_at, updated_at
		FROM clients 
		WHERE id = ?
	`

	row := r.db.QueryRow(query, id)

	var client domain.Client
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&client.ID,
		&client.TelegramID,
		&client.FIO,
		&client.Contact,
		&client.Address,
		&client.Latitude,
		&client.Longitude,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	client.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	client.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

	return &client, nil
}

// GetAll retrieves all clients
func (r *ClientRepository) GetAll() ([]domain.Client, error) {
	query := `
		SELECT id, telegram_id, fio, contact, address, latitude, longitude, created_at, updated_at
		FROM clients 
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []domain.Client

	for rows.Next() {
		var client domain.Client
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&client.ID,
			&client.TelegramID,
			&client.FIO,
			&client.Contact,
			&client.Address,
			&client.Latitude,
			&client.Longitude,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		client.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		client.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		clients = append(clients, client)
	}

	return clients, nil
}

// Delete removes a client by ID
func (r *ClientRepository) Delete(id int64) error {
	query := "DELETE FROM clients WHERE id = ?"
	_, err := r.db.Exec(query, id)
	return err
}
