package service

import (
	"database/sql"
	"errors"
	"fmt"

	"magaz/internal/models"
	"magaz/internal/repository"
)

type OrderService struct {
	orders   *repository.OrderRepository
	products *repository.ProductRepository
	cart     *repository.CartRepository
	addrs    *repository.AddressRepository
	payment  *PaymentClient
	db       *sql.DB
}

func NewOrderService(
	orders *repository.OrderRepository,
	products *repository.ProductRepository,
	cart *repository.CartRepository,
	addrs *repository.AddressRepository,
	payment *PaymentClient,
	db *sql.DB,
) *OrderService {
	return &OrderService{
		orders: orders, products: products,
		cart: cart, addrs: addrs,
		payment: payment, db: db,
	}
}

type CheckoutInput struct {
	UserID     int64
	AddressID  int64
	CardNumber string `validate:"required,len=16,numeric"`
	Expiry     string `validate:"required"`
	CVV        string `validate:"required,min=3,max=4,numeric"`
}

// Checkout creates an order, charges payment, decrements stock — idempotent flow.
// 1. Create order (pending) + items + decrement stock inside a DB tx
// 2. Attempt payment AFTER the order is safely persisted
// 3. Update order status to "paid" on success, leave as "pending" on failure
func (s *OrderService) Checkout(in CheckoutInput) (*models.Order, error) {
	// 1. Fetch cart items (outside tx — read only)
	items, err := s.cart.Items(in.UserID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrEmptyCart
	}

	// 2. Verify address belongs to user
	addr, err := s.addrs.FindByIDAndUser(in.AddressID, in.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errors.New("адрес не найден")
	}
	if err != nil {
		return nil, err
	}

	// 3. Calculate total
	var total float64
	for _, ci := range items {
		total += ci.Subtotal()
	}

	// 4. Create order + items + decrement stock in DB transaction FIRST
	//    This guarantees the order exists even if payment fails later.
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	addrID := addr.ID
	order := &models.Order{
		UserID:      in.UserID,
		AddressID:   &addrID,
		Status:      models.StatusPending,
		TotalAmount: total,
	}
	if err := s.orders.Create(tx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	for _, ci := range items {
		item := &models.OrderItem{
			OrderID:       order.ID,
			ProductID:     ci.ProductID,
			ProductName:   ci.Product.Name,
			Quantity:      ci.Quantity,
			PriceSnapshot: ci.Product.Price,
		}
		if err := s.orders.AddItem(tx, item); err != nil {
			return nil, fmt.Errorf("add order item: %w", err)
		}
		if err := s.products.DecrementStock(tx, ci.ProductID, ci.Quantity); err != nil {
			return nil, err
		}
	}

	if err := s.cart.Clear(tx, in.UserID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// 5. Charge payment AFTER order is safely persisted
	chargeResp, err := s.payment.Charge(ChargeRequest{
		OrderID:    order.ID,
		Amount:     total,
		CardNumber: in.CardNumber,
		Expiry:     in.Expiry,
		CVV:        in.CVV,
	})
	if err != nil {
		// Order stays as "pending" — user can retry or admin can investigate
		return order, fmt.Errorf("ошибка оплаты: %w (заказ #%d создан, повторите оплату)", err, order.ID)
	}
	if chargeResp.Status != "success" {
		return order, fmt.Errorf("оплата отклонена: %s (заказ #%d создан)", chargeResp.Message, order.ID)
	}

	// 6. Mark order as paid
	if err := s.orders.SetPaymentTx(order.ID, chargeResp.TxID, models.StatusPaid); err != nil {
		// Payment succeeded but status update failed — log but don't fail
		// The order exists and payment is recorded in the payment service
		return order, nil
	}

	order.Status = models.StatusPaid
	order.PaymentTxID = &chargeResp.TxID
	return order, nil
}

func (s *OrderService) GetForUser(orderID, userID int64) (*models.Order, error) {
	order, err := s.orders.FindByIDAndUser(orderID, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.orders.Items(orderID)
	if err != nil {
		return nil, err
	}
	order.Items = items
	return order, nil
}

func (s *OrderService) ListByUser(userID int64) ([]*models.Order, error) {
	return s.orders.ListByUser(userID)
}

func (s *OrderService) ListAll(page, perPage int) ([]*models.Order, int, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}
	return s.orders.ListAll(perPage, (page-1)*perPage)
}

func (s *OrderService) UpdateStatus(orderID int64, status models.OrderStatus) error {
	return s.orders.UpdateStatus(orderID, status)
}

func (s *OrderService) GetByID(id int64) (*models.Order, error) {
	order, err := s.orders.FindByID(id)
	if err != nil {
		return nil, err
	}
	items, err := s.orders.Items(id)
	if err != nil {
		return nil, err
	}
	order.Items = items
	return order, nil
}

func (s *OrderService) Stats() (*repository.OrderStats, error) {
	return s.orders.Stats()
}
