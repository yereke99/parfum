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
		INSERT INTO orders (id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := r.db.Exec(query,
		order.IDUser,
		order.UserName,
		order.Quantity,
		order.Parfumes,
		order.FIO,
		order.Contact,
		order.Address,
		order.DateRegister,
		order.DataPay,
		order.Checks)

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
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE id = ?
	`

	row := r.db.QueryRow(query, id)

	var order domain.Order
	var createdAt, updatedAt time.Time
	var parfumes, fio, address, dateRegister sql.NullString

	err := row.Scan(
		&order.ID,
		&order.IDUser,
		&order.UserName,
		&order.Quantity,
		&parfumes,
		&fio,
		&order.Contact,
		&address,
		&dateRegister,
		&order.DataPay,
		&order.Checks,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if parfumes.Valid {
		order.Parfumes = parfumes.String
	}
	if fio.Valid {
		order.FIO = fio.String
	}
	if address.Valid {
		order.Address = address.String
	}
	if dateRegister.Valid {
		order.DateRegister = dateRegister.String
	}

	order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

	return &order, nil
}

// GetByUserID retrieves orders by user ID
func (r *OrderRepository) GetByUserID(userID int64) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE id_user = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&order.Quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
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
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
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
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&order.Quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// UpdateChecks updates order check status
func (r *OrderRepository) UpdateChecks(id int64, checks bool) error {
	query := `
		UPDATE orders 
		SET checks = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query, checks, id)
	return err
}

// UpdatePaymentDate updates the payment date
func (r *OrderRepository) UpdatePaymentDate(id int64, dataPay string) error {
	query := `
		UPDATE orders 
		SET dataPay = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query, dataPay, id)
	return err
}

// Update updates an order
func (r *OrderRepository) Update(order *domain.Order) error {
	query := `
		UPDATE orders 
		SET id_user = ?, userName = ?, quantity = ?, parfumes = ?, fio = ?, 
		    contact = ?, address = ?, dateRegister = ?, dataPay = ?, checks = ?, 
		    updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		order.IDUser,
		order.UserName,
		order.Quantity,
		order.Parfumes,
		order.FIO,
		order.Contact,
		order.Address,
		order.DateRegister,
		order.DataPay,
		order.Checks,
		order.ID)

	return err
}

// Delete removes an order by ID
func (r *OrderRepository) Delete(id int64) error {
	query := "DELETE FROM orders WHERE id = ?"
	_, err := r.db.Exec(query, id)
	return err
}

// GetOrdersByChecksStatus retrieves orders by check status
func (r *OrderRepository) GetOrdersByChecksStatus(checks bool) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE checks = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, checks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&order.Quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetOrdersByUserName retrieves orders by username
func (r *OrderRepository) GetOrdersByUserName(userName string) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE userName LIKE ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, "%"+userName+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&order.Quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
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

	// Pending orders (unchecked)
	var pendingOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE checks = 0").Scan(&pendingOrders)
	if err != nil {
		return nil, err
	}
	stats["pending_orders"] = pendingOrders

	// Completed orders (checked)
	var completedOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE checks = 1").Scan(&completedOrders)
	if err != nil {
		return nil, err
	}
	stats["completed_orders"] = completedOrders

	// Total quantity
	var totalQuantity sql.NullInt64
	err = r.db.QueryRow("SELECT SUM(quantity) FROM orders").Scan(&totalQuantity)
	if err != nil {
		return nil, err
	}
	if totalQuantity.Valid {
		stats["total_quantity"] = totalQuantity.Int64
	} else {
		stats["total_quantity"] = 0
	}

	// Today's orders
	var todayOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE DATE(created_at) = DATE('now')").Scan(&todayOrders)
	if err != nil {
		return nil, err
	}
	stats["today_orders"] = todayOrders

	// This week's orders
	var weekOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at >= datetime('now', '-7 days')").Scan(&weekOrders)
	if err != nil {
		return nil, err
	}
	stats["week_orders"] = weekOrders

	// This month's orders
	var monthOrders int
	err = r.db.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at >= datetime('now', 'start of month')").Scan(&monthOrders)
	if err != nil {
		return nil, err
	}
	stats["month_orders"] = monthOrders

	return stats, nil
}

// GetOrdersByDateRange retrieves orders within a date range
func (r *OrderRepository) GetOrdersByDateRange(startDate, endDate string) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE DATE(created_at) BETWEEN ? AND ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order

	for rows.Next() {
		var order domain.Order
		var createdAt, updatedAt time.Time
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&order.Quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// CountOrdersByUser returns the count of orders for a specific user
func (r *OrderRepository) CountOrdersByUser(userID int64) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM orders WHERE id_user = ?"
	err := r.db.QueryRow(query, userID).Scan(&count)
	return count, err
}

// Add these methods to your OrderRepository

// GetUnpaidOrdersByUser gets all unpaid orders for a user
func (r *OrderRepository) GetUnpaidOrdersByUser(telegramID int64) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE id_user = ? AND checks = 0 AND quantity > 0
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
		var quantity sql.NullInt64
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if quantity.Valid {
			qty := int(quantity.Int64)
			order.Quantity = &qty
		}
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetAvailableQuantityForUser calculates available perfume quantity for user
func (r *OrderRepository) GetAvailableQuantityForUser(telegramID int64) (int, error) {
	query := `
		SELECT 
			COALESCE(SUM(
				CASE 
					WHEN quantity IS NULL THEN 0
					ELSE quantity - (
						CASE 
							WHEN parfumes IS NULL OR parfumes = '' THEN 0
							ELSE (LENGTH(parfumes) - LENGTH(REPLACE(parfumes, ':', '')))/1
						END
					)
				END
			), 0) as available
		FROM orders 
		WHERE id_user = ? AND checks = 0 AND quantity > 0
	`

	var available int
	err := r.db.QueryRow(query, telegramID).Scan(&available)
	if err != nil {
		return 0, err
	}

	return available, nil
}

// UpdatePerfumeSelection updates the parfumes field for an order
func (r *OrderRepository) UpdatePerfumeSelection(orderID int64, parfumes string) error {
	query := `
		UPDATE orders 
		SET parfumes = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query, parfumes, orderID)
	return err
}

// GetOrderWithPerfumeSelection gets an order that has perfume selection but no client info yet
func (r *OrderRepository) GetOrderWithPerfumeSelection(telegramID int64) (*domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE id_user = ? AND checks = 0 AND parfumes IS NOT NULL AND parfumes != ''
		ORDER BY updated_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(query, telegramID)

	var order domain.Order
	var createdAt, updatedAt time.Time
	var quantity sql.NullInt64
	var parfumes, fio, address, dateRegister sql.NullString

	err := row.Scan(
		&order.ID,
		&order.IDUser,
		&order.UserName,
		&quantity,
		&parfumes,
		&fio,
		&order.Contact,
		&address,
		&dateRegister,
		&order.DataPay,
		&order.Checks,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if quantity.Valid {
		qty := int(quantity.Int64)
		order.Quantity = &qty
	}
	if parfumes.Valid {
		order.Parfumes = parfumes.String
	}
	if fio.Valid {
		order.FIO = fio.String
	}
	if address.Valid {
		order.Address = address.String
	}
	if dateRegister.Valid {
		order.DateRegister = dateRegister.String
	}

	order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

	return &order, nil
}

// UpdateClientInfo updates order with client information
func (r *OrderRepository) UpdateClientInfo(orderID int64, fio, contact, address string) error {
	query := `
		UPDATE orders 
		SET fio = ?, contact = ?, address = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`

	_, err := r.db.Exec(query, fio, contact, address, orderID)
	return err
}

// GetOrdersByUserWithSelection gets orders with perfume selections for a user
func (r *OrderRepository) GetOrdersByUserWithSelection(telegramID int64) ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE id_user = ? AND checks = 0 AND parfumes IS NOT NULL AND parfumes != ''
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
		var quantity sql.NullInt64
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if quantity.Valid {
			qty := int(quantity.Int64)
			order.Quantity = &qty
		}
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetUncompletedOrdersWithPerfumes gets orders that have perfume selection but incomplete client info
func (r *OrderRepository) GetUncompletedOrdersWithPerfumes() ([]domain.Order, error) {
	query := `
		SELECT id, id_user, userName, quantity, parfumes, fio, contact, address, dateRegister, dataPay, checks, created_at, updated_at
		FROM orders 
		WHERE checks = 0 
		AND parfumes IS NOT NULL 
		AND parfumes != ''
		AND (fio IS NULL OR fio = '' OR address IS NULL OR address = '')
		ORDER BY updated_at DESC
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
		var quantity sql.NullInt64
		var parfumes, fio, address, dateRegister sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.IDUser,
			&order.UserName,
			&quantity,
			&parfumes,
			&fio,
			&order.Contact,
			&address,
			&dateRegister,
			&order.DataPay,
			&order.Checks,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if quantity.Valid {
			qty := int(quantity.Int64)
			order.Quantity = &qty
		}
		if parfumes.Valid {
			order.Parfumes = parfumes.String
		}
		if fio.Valid {
			order.FIO = fio.String
		}
		if address.Valid {
			order.Address = address.String
		}
		if dateRegister.Valid {
			order.DateRegister = dateRegister.String
		}

		order.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		order.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")

		orders = append(orders, order)
	}

	return orders, nil
}

// GetPendingOrdersCount returns count of pending orders
func (r *OrderRepository) GetPendingOrdersCount() (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM orders WHERE checks = 0"
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// GetCompletedOrdersCount returns count of completed orders
func (r *OrderRepository) GetCompletedOrdersCount() (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM orders WHERE checks = 1"
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// GetOrdersWithPerfumeSelectionCount returns count of orders that have perfume selections
func (r *OrderRepository) GetOrdersWithPerfumeSelectionCount() (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM orders WHERE parfumes IS NOT NULL AND parfumes != ''"
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

// GetTotalQuantityOrdered returns total quantity of all orders
func (r *OrderRepository) GetTotalQuantityOrdered() (int, error) {
	var total sql.NullInt64
	query := "SELECT SUM(quantity) FROM orders WHERE quantity IS NOT NULL"
	err := r.db.QueryRow(query).Scan(&total)
	if err != nil {
		return 0, err
	}

	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}

// UpdateClientInfoWithCoordinates updates order with client info and optional coordinates
func (r *OrderRepository) UpdateClientInfoWithCoordinates(orderID int64, fio, contact, address string) error {
	query := `
		UPDATE orders 
		SET fio = ?, contact = ?, address = ?,  updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.db.Exec(query, fio, contact, address, orderID)
	return err
}

// Add coordinates to existing order
func (r *OrderRepository) UpdateOrderCoordinates(orderID int64, latitude, longitude float64) error {
	query := `
		UPDATE orders 
		SET latitude = ?, longitude = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.db.Exec(query, latitude, longitude, orderID)
	return err
}
