package domain

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
