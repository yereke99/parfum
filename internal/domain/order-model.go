package domain

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
