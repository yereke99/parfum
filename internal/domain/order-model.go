package domain

import (
	"database/sql"
	"fmt"
	"time"
)

// OrderEntry — запись из списка заказов (как она читается из БД)
type OrderEntry struct {
	ID           int64          `json:"id"            db:"id"`
	UserID       int64          `json:"userID"        db:"id_user"`
	UserName     string         `json:"userName"      db:"userName"`
	Parfumes     string         `json:"parfumes"      db:"parfumes"`
	Quantity     int            `json:"quantity"      db:"quantity"`
	Fio          sql.NullString `json:"fio"           db:"fio"`
	Contact      string         `json:"contact"       db:"contact"`
	Address      sql.NullString `json:"address"       db:"address"`
	DateRegister sql.NullString `json:"dateRegister"  db:"dateRegister"`
	DatePay      string         `json:"dataPay"       db:"dataPay"` // имя поля — DatePay, но ключи — dataPay
	Checks       bool           `json:"checks"        db:"checks"`
}

// Order — полная доменная модель заказа
type Order struct {
	ID           int64     `json:"id"            db:"id"`
	IDUser       int64     `json:"id_user"       db:"id_user"`
	UserName     string    `json:"userName"      db:"userName"`
	Quantity     *int      `json:"quantity"      db:"quantity"`
	Parfumes     string    `json:"parfumes"      db:"parfumes"`
	Gift         string    `json:"gift"          db:"gift"`
	FIO          string    `json:"fio"           db:"fio"`
	Contact      string    `json:"contact"       db:"contact"`
	Address      string    `json:"address"       db:"address"`
	DateRegister string    `json:"dateRegister"  db:"dateRegister"`
	DataPay      string    `json:"dataPay"       db:"dataPay"` // ЕДИНЫЙ нейминг: DataPay
	Checks       bool      `json:"checks"        db:"checks"`
	CreatedAt    time.Time `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"    db:"updated_at"`
}

// OrderCreateRequest — вход при создании
type OrderCreateRequest struct {
	IDUser       int64  `json:"id_user"      validate:"required"`
	UserName     string `json:"userName"     validate:"required,min=1,max=255"`
	Quantity     *int   `json:"quantity"`
	Parfumes     string `json:"parfumes"`
	FIO          string `json:"fio"`
	Contact      string `json:"contact"      validate:"required,min=1,max=50"`
	Address      string `json:"address"`
	DateRegister string `json:"dateRegister"`
	DataPay      string `json:"dataPay"      validate:"required"`
	Checks       bool   `json:"checks"`
}

// OrderUpdateRequest — частичное обновление
type OrderUpdateRequest struct {
	UserName     string `json:"userName,omitempty"     validate:"omitempty,min=1,max=255"`
	Quantity     *int   `json:"quantity,omitempty"`
	Parfumes     string `json:"parfumes,omitempty"`
	FIO          string `json:"fio,omitempty"`
	Contact      string `json:"contact,omitempty"      validate:"omitempty,min=1,max=50"`
	Address      string `json:"address,omitempty"`
	DateRegister string `json:"dateRegister,omitempty"`
	DataPay      string `json:"dataPay,omitempty"`
	Checks       *bool  `json:"checks,omitempty"`
}

// OrderResponse — то, что отдаём наружу
type OrderResponse struct {
	ID           int64  `json:"id"`
	IDUser       int64  `json:"id_user"`
	UserName     string `json:"userName"`
	Quantity     *int   `json:"quantity"`
	Parfumes     string `json:"parfumes"`
	FIO          string `json:"fio"`
	Contact      string `json:"contact"`
	Address      string `json:"address"`
	DateRegister string `json:"dateRegister"`
	DataPay      string `json:"dataPay"`
	Checks       bool   `json:"checks"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// OrderStatsResponse — статистика по заказам
type OrderStatsResponse struct {
	TotalOrders     int `json:"total_orders"`
	PendingOrders   int `json:"pending_orders"`
	CompletedOrders int `json:"completed_orders"`
	TotalQuantity   int `json:"total_quantity"`
	TodayOrders     int `json:"today_orders"`
	WeekOrders      int `json:"week_orders"`
	MonthOrders     int `json:"month_orders"`
}

// ToResponse — маппинг доменной модели в внешний ответ
func (o *Order) ToResponse() *OrderResponse {
	return &OrderResponse{
		ID:           o.ID,
		IDUser:       o.IDUser,
		UserName:     o.UserName,
		Quantity:     o.Quantity,
		Parfumes:     o.Parfumes,
		FIO:          o.FIO,
		Contact:      o.Contact,
		Address:      o.Address,
		DateRegister: o.DateRegister,
		DataPay:      o.DataPay,
		Checks:       o.Checks,
		CreatedAt:    o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    o.UpdatedAt.Format(time.RFC3339),
	}
}

// FromCreateRequest — заполнение из create-запроса
func (o *Order) FromCreateRequest(req *OrderCreateRequest) {
	o.IDUser = req.IDUser
	o.UserName = req.UserName
	o.Quantity = req.Quantity
	o.Parfumes = req.Parfumes
	o.FIO = req.FIO
	o.Contact = req.Contact
	o.Address = req.Address
	o.DateRegister = req.DateRegister
	o.DataPay = req.DataPay
	o.Checks = req.Checks
}

// UpdateFromRequest — частичное обновление из update-запроса
func (o *Order) UpdateFromRequest(req *OrderUpdateRequest) {
	if req.UserName != "" {
		o.UserName = req.UserName
	}
	if req.Quantity != nil {
		o.Quantity = req.Quantity
	}
	if req.Parfumes != "" {
		o.Parfumes = req.Parfumes
	}
	if req.FIO != "" {
		o.FIO = req.FIO
	}
	if req.Contact != "" {
		o.Contact = req.Contact
	}
	if req.Address != "" {
		o.Address = req.Address
	}
	if req.DateRegister != "" {
		o.DateRegister = req.DateRegister
	}
	if req.DataPay != "" {
		o.DataPay = req.DataPay
	}
	if req.Checks != nil {
		o.Checks = *req.Checks
	}
}

// IsValid — простая валидация доменной модели
func (o *Order) IsValid() error {
	if o.IDUser == 0 {
		return fmt.Errorf("id_user is required")
	}
	if o.UserName == "" {
		return fmt.Errorf("userName is required")
	}
	if o.Contact == "" {
		return fmt.Errorf("contact is required")
	}
	if o.DataPay == "" {
		return fmt.Errorf("dataPay is required")
	}
	return nil
}
