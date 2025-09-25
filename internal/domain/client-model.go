package domain

import "database/sql"

type Client struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	FIO        string `json:"fio"`
	Contact    string `json:"contact"`
	Address    string `json:"address"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ClientEntry represents a paying client in the client table
type ClientEntry struct {
	ID           int64          `json:"id" db:"id"`
	UserID       int64          `json:"userID" db:"id_user"`
	UserName     string         `json:"userName" db:"userName"`
	Fio          sql.NullString `json:"fio" db:"fio"`
	Contact      string         `json:"contact" db:"contact"`
	Address      sql.NullString `json:"address" db:"address"`
	DateRegister sql.NullString `json:"dateRegister" db:"dateRegister"`
	DatePay      string         `json:"dataPay" db:"dataPay"`
	Checks       bool           `json:"checks" db:"checks"`
}

// Update your existing LotoEntry struct to include Checks field
type LotoEntry struct {
	UserID    int64          `json:"user_id" db:"id_user"`
	LotoID    int            `json:"loto_id" db:"id_loto"`
	QR        string         `json:"qr" db:"qr"`
	WhoPaid   sql.NullString `json:"who_paid" db:"who_paid"`
	Receipt   string         `json:"receipt" db:"receipt"`
	Fio       sql.NullString `json:"fio" db:"fio"`
	Contact   sql.NullString `json:"contact" db:"contact"`
	Address   sql.NullString `json:"address" db:"address"`
	DatePay   string         `json:"date_pay" db:"dataPay"`
	UpdatedAt string         `json:"updated_at" db:"updated_at"`
	Checks    bool           `json:"checks" db:"checks"` // Add this field
}
