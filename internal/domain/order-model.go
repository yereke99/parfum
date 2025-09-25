package domain

import "database/sql"

type Order struct {
	ID          int64  `json:"id"`
	TelegramID  int64  `json:"telegram_id"`
	ClientID    int64  `json:"client_id"`
	CartData    string `json:"cart_data"`
	TotalAmount int    `json:"total_amount"`
	Status      string `json:"status"`
	PaymentLink string `json:"payment_link"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type OrderEntry struct {
	ID           int64          `json:"id" db:"id"`
	UserID       int64          `json:"userID" db:"id_user"`
	UserName     string         `json:"userName" db:"userName"`
	Quantity     int            `json:"quantity" db:"quantity"`
	Fio          sql.NullString `json:"fio" db:"fio"`
	Contact      string         `json:"contact" db:"contact"`
	Address      sql.NullString `json:"address" db:"address"`
	DateRegister sql.NullString `json:"dateRegister" db:"dateRegister"`
	DatePay      string         `json:"dataPay" db:"dataPay"`
	Checks       bool           `json:"checks" db:"checks"`
}
