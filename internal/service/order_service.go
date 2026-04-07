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

// Checkout creates an order, charges payment, decrements stock — all in one DB tx.
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

	// 4. Charge payment BEFORE touching the DB (avoid phantom orders)
	chargeResp, err := s.payment.Charge(ChargeRequest{
		OrderID:    0, // will be filled after order creation
		Amount:     total,
		CardNumber: in.CardNumber,
		Expiry:     in.Expiry,
		CVV:        in.CVV,
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка оплаты: %w", err)
	}
	if chargeResp.Status != "success" {
		return nil, fmt.Errorf("оплата отклонена: %s", chargeResp.Message)
	}

	// 5. Create order + items in DB transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	addrID := addr.ID
	order := &models.Order{
		UserID:      in.UserID,
		AddressID:   &addrID,
		Status:      models.StatusPaid,
		TotalAmount: total,
		PaymentTxID: &chargeResp.TxID,
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
