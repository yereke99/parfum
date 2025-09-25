package repository

import (
	"context"
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

// ExistsJust проверяет, есть ли запись в just по id_user
func (r *ClientRepository) ExistsJust(ctx context.Context, userId int64) (bool, error) {
	const q = `SELECT COUNT(1) FROM just WHERE id_user=?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, userId).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ExistsClient проверяет, есть ли запись в client по id_user
func (r *ClientRepository) ExistsClient(ctx context.Context, userID int64) (bool, error) {
	const q = `SELECT COUNT(1) FROM client WHERE id_user = ?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ExistsLoto проверяет, есть ли запись в loto по id_user
func (r *ClientRepository) ExistsLoto(ctx context.Context, userID int64) (bool, error) {
	const q = `SELECT COUNT(1) FROM loto WHERE id_user = ?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ExistsGeo проверяет, есть ли запись в geo по id_user
func (r *ClientRepository) ExistsGeo(ctx context.Context, userID int64) (bool, error) {
	const q = `SELECT COUNT(1) FROM geo WHERE id_user = ?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// IsClientPaid проверяет, оплачен ли клиент
func (r *ClientRepository) IsClientPaid(ctx context.Context, userID int64) (bool, error) {
	const q = `SELECT checks FROM client WHERE id_user = ?;`
	var checks bool
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&checks)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return checks, nil
}

// InsertJust вставляет запись в таблицу just с учетом новых полей (SQLite version)
func (r *ClientRepository) InsertJust(ctx context.Context, e domain.JustEntry) error {
	const q = `
		INSERT OR REPLACE INTO just (id_user, userName, dataRegistred, updated_at)
		VALUES (?, ?, ?, datetime('now'));
	`
	_, err := r.db.ExecContext(ctx, q, e.UserId, e.UserName, e.DateRegistered)
	return err
}

// InsertClient вставляет запись в таблицу client с учетом новых полей (SQLite version)
func (r *ClientRepository) InsertClient(ctx context.Context, e domain.ClientEntry) error {
	const q = `
		INSERT OR REPLACE INTO client (id_user, userName, fio, contact, address, dateRegister, dataPay, checks, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'));
	`
	_, err := r.db.ExecContext(ctx, q,
		e.UserID, e.UserName, e.Fio, e.Contact,
		e.Address, e.DateRegister, e.DatePay, e.Checks,
	)
	return err
}

func (r *ClientRepository) IsUniqueQr(ctx context.Context, qr string) (bool, error) {
	const q = `SELECT COUNT(1) FROM loto WHERE qr = ?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, qr).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// IncreaseTotalSum increases the total sum by the specified amount
func (r *ClientRepository) IncreaseTotalSum(ctx context.Context, amount int) error {
	const q = `UPDATE money SET sum = sum + ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1;`
	_, err := r.db.ExecContext(ctx, q, amount)
	return err
}

// InsertLoto inserts loto entry with updated domain model
func (r *ClientRepository) InsertLoto(ctx context.Context, e domain.LotoEntry) error {
	const q = `
		INSERT OR REPLACE INTO loto (id_user, id_loto, qr, who_paid, receipt, fio, contact, address, dataPay, checks, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'));
	`
	_, err := r.db.ExecContext(ctx, q,
		e.UserID, e.LotoID, e.QR, e.WhoPaid,
		e.Receipt, e.Fio, e.Contact, e.Address, e.DatePay, e.Checks,
	)
	return err
}

func (r *ClientRepository) InsertOrder(ctx context.Context, order domain.OrderEntry) error {
	const q = `
		INSERT INTO orders (id_user, userName, quantity, fio, contact, address, dateRegister, dataPay, checks)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	_, err := r.db.ExecContext(ctx, q,
		order.UserID,
		order.UserName,
		order.Quantity,
		order.Fio,
		order.Contact,
		order.Address,
		order.DateRegister,
		order.DatePay,
		order.Checks,
	)
	return err
}

// IsClientUnique возвращает true, если в client нет записи с данным id_user
func (r *ClientRepository) IsClientUnique(ctx context.Context, userID int64) (bool, error) {
	const q = `SELECT COUNT(1) FROM client WHERE id_user = ?;`
	var cnt int
	if err := r.db.QueryRowContext(ctx, q, userID).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt == 0, nil
}
