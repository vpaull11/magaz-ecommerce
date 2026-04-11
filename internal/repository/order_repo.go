package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"magaz/internal/models"
)

type OrderRepository struct{ db *sql.DB }

func NewOrderRepository(db *sql.DB) *OrderRepository { return &OrderRepository{db: db} }

func (r *OrderRepository) DB() *sql.DB { return r.db }

func (r *OrderRepository) Create(tx *sql.Tx, o *models.Order) error {
	return tx.QueryRow(
		`INSERT INTO orders (user_id, address_id, status, total_amount)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		o.UserID, o.AddressID, o.Status, o.TotalAmount,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
}

func (r *OrderRepository) AddItem(tx *sql.Tx, item *models.OrderItem) error {
	_, err := tx.Exec(
		`INSERT INTO order_items (order_id, product_id, product_name, quantity, price_snapshot)
		 VALUES ($1,$2,$3,$4,$5)`,
		item.OrderID, item.ProductID, item.ProductName, item.Quantity, item.PriceSnapshot,
	)
	return err
}

func (r *OrderRepository) SetPaymentTx(orderID int64, txID string, status models.OrderStatus) error {
	_, err := r.db.Exec(
		`UPDATE orders SET payment_tx_id=$1, status=$2, updated_at=NOW() WHERE id=$3`,
		txID, status, orderID,
	)
	return err
}

func (r *OrderRepository) UpdateStatus(orderID int64, status models.OrderStatus) error {
	_, err := r.db.Exec(
		`UPDATE orders SET status=$1, updated_at=NOW() WHERE id=$2`,
		status, orderID,
	)
	return err
}

func (r *OrderRepository) FindByID(id int64) (*models.Order, error) {
	o := &models.Order{}
	err := r.db.QueryRow(
		`SELECT id, user_id, address_id, status, total_amount, payment_tx_id, created_at, updated_at
		 FROM orders WHERE id=$1`, id,
	).Scan(&o.ID, &o.UserID, &o.AddressID, &o.Status, &o.TotalAmount,
		&o.PaymentTxID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (r *OrderRepository) FindByIDAndUser(id, userID int64) (*models.Order, error) {
	o := &models.Order{}
	err := r.db.QueryRow(
		`SELECT id, user_id, address_id, status, total_amount, payment_tx_id, created_at, updated_at
		 FROM orders WHERE id=$1 AND user_id=$2`, id, userID,
	).Scan(&o.ID, &o.UserID, &o.AddressID, &o.Status, &o.TotalAmount,
		&o.PaymentTxID, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
}

func (r *OrderRepository) Items(orderID int64) ([]models.OrderItem, error) {
	rows, err := r.db.Query(
		`SELECT id, order_id, product_id, product_name, quantity, price_snapshot
		 FROM order_items WHERE order_id=$1`, orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID,
			&item.ProductName, &item.Quantity, &item.PriceSnapshot); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *OrderRepository) ListByUser(userID int64) ([]*models.Order, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, address_id, status, total_amount, payment_tx_id, created_at, updated_at
		 FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (r *OrderRepository) ListAll(limit, offset int) ([]*models.Order, int, error) {
	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM orders`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(
		`SELECT id, user_id, address_id, status, total_amount, payment_tx_id, created_at, updated_at
		 FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	orders, err := scanOrders(rows)
	return orders, total, err
}

func scanOrders(rows *sql.Rows) ([]*models.Order, error) {
	var orders []*models.Order
	for rows.Next() {
		o := &models.Order{}
		if err := rows.Scan(&o.ID, &o.UserID, &o.AddressID, &o.Status,
			&o.TotalAmount, &o.PaymentTxID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// OrderStats holds aggregated stats for the admin dashboard.
type OrderStats struct {
	Revenue30d float64
	Orders30d  int
}

// Stats returns aggregated order metrics for the last 30 days.
func (r *OrderRepository) Stats() (*OrderStats, error) {
	s := &OrderStats{}
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(total_amount), 0), COUNT(*)
		 FROM orders
		 WHERE created_at >= NOW() - INTERVAL '30 days'
		   AND status IN ('paid', 'shipped', 'done')`,
	).Scan(&s.Revenue30d, &s.Orders30d)
	return s, err
}
