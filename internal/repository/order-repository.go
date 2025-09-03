package repository

import (
	"database/sql"
	"parfum/internal/domain"
	"time"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create creates a new order
func (r *OrderRepository) Create(order *domain.Order) error {
	query := `
		INSERT INTO orders (telegram_id, client_id, cart_data, total_amount, status, payment_link, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := r.db.Exec(query,
		order.TelegramID,
		order.ClientID,
		order.CartData,
		order.TotalAmount,
		order.Status,
		order.PaymentLink)

	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	order.ID = id
	return nil
}

// GetByID retrieves an order by ID
func (r *OrderRepository) GetByID(id int64) (*domain.Order, error) {
	query := `
		SELECT id, telegram_id, client_id, cart_data, total_amount, status, payment_link, created_at, updated_at
		FROM orders 
		WHERE id = ?
	`

	row := r.db.QueryRow(query, id)

	var order domain.Order
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&order.ID,
		&order.TelegramID,
		&order.ClientID,
		&order.CartData,
		&order.TotalAmount,
		&order.Status,
		&order.PaymentLink,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

	return &order, nil
}

// GetByTelegramID retrieves orders by telegram ID
func (r *OrderRepository) GetByTelegramID(telegramID int64) ([]domain.Order, error) {
	query := `
		SELECT id, telegram_id, client_id, cart_data, total_amount, status, payment_link, created_at, updated_at
		FROM orders 
		WHERE telegram_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, telegramID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&order.ID,
			&order.TelegramID,
			&order.ClientID,
			&order.CartData,
			&order.TotalAmount,
			&order.Status,
			&order.PaymentLink,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetAll retrieves all orders
func (r *OrderRepository) GetAll() ([]domain.Order, error) {
	query := `
		SELECT id, telegram_id, client_id, cart_data, total_amount, status, payment_link, created_at, updated_at
		FROM orders 
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&order.ID,
			&order.TelegramID,
			&order.ClientID,
			&order.CartData,
			&order.TotalAmount,
			&order.Status,
			&order.PaymentLink,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// UpdateStatus updates order status
func (r *OrderRepository) UpdateStatus(id int64, status string) error {
	query := `
		UPDATE orders 
		SET status = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query, status, id)
	return err
}

// Update updates an order
func (r *OrderRepository) Update(order *domain.Order) error {
	query := `
		UPDATE orders 
		SET telegram_id = ?, client_id = ?, cart_data = ?, total_amount = ?, status = ?, 
		    payment_link = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		order.TelegramID,
		order.ClientID,
		order.CartData,
		order.TotalAmount,
		order.Status,
		order.PaymentLink,
		order.ID)

	return err
}

// Delete removes an order by ID
func (r *OrderRepository) Delete(id int64) error {
	query := "DELETE FROM orders WHERE id = ?"
	_, err := r.db.Exec(query, id)
	return err
}

// GetOrdersByStatus retrieves orders by status
func (r *OrderRepository) GetOrdersByStatus(status string) ([]domain.Order, error) {
	query := `
		SELECT id, telegram_id, client_id, cart_data, total_amount, status, payment_link, created_at, updated_at
		FROM orders 
		WHERE status = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&order.ID,
			&order.TelegramID,
			&order.ClientID,
			&order.CartData,
			&order.TotalAmount,
			&order.Status,
			&order.PaymentLink,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetOrderStats returns order statistics
func (r *OrderRepository) GetOrderStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total orders
	var totalOrders int
	err := r.db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&totalOrders)
	if err != nil {
		return nil, err
	}
	stats["total_orders"] = totalOrders

	// Pending orders
	var pendingOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE status = 'pending'").Scan(&pendingOrders)
	if err != nil {
		return nil, err
	}
	stats["pending_orders"] = pendingOrders

	// Completed orders
	var completedOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE status = 'completed'").Scan(&completedOrders)
	if err != nil {
		return nil, err
	}
	stats["completed_orders"] = completedOrders

	// Total revenue
	var totalRevenue sql.NullInt64
	err = r.db.QueryRow("SELECT SUM(total_amount) FROM orders WHERE status = 'completed'").Scan(&totalRevenue)
	if err != nil {
		return nil, err
	}
	if totalRevenue.Valid {
		stats["total_revenue"] = totalRevenue.Int64
	} else {
		stats["total_revenue"] = 0
	}

	// Today's orders
	var todayOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE DATE(created_at) = DATE('now')").Scan(&todayOrders)
	if err != nil {
		return nil, err
	}
	stats["today_orders"] = todayOrders

	return stats, nil
}
