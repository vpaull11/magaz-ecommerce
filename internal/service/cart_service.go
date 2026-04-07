package service

import (
	"errors"

	"magaz/internal/models"
	"magaz/internal/repository"
)

var ErrEmptyCart = errors.New("корзина пуста")

type CartService struct {
	cart     *repository.CartRepository
	products *repository.ProductRepository
}

func NewCartService(
	cart *repository.CartRepository,
	products *repository.ProductRepository,
) *CartService {
	return &CartService{cart: cart, products: products}
}

type CartSummary struct {
	Items []CartLineItem
	Total float64
	Count int
}

type CartLineItem struct {
	Item    *models.CartItem
	Product *models.Product
}

func (s *CartService) Get(userID int64) (*CartSummary, error) {
	items, err := s.cart.Items(userID)
	if err != nil {
		return nil, err
	}
	summary := &CartSummary{}
	for _, ci := range items {
		summary.Items = append(summary.Items, CartLineItem{Item: ci, Product: ci.Product})
		summary.Total += ci.Subtotal()
	}
	summary.Count = len(summary.Items)
	return summary, nil
}

func (s *CartService) Add(userID, productID int64, qty int) error {
	if qty <= 0 {
		qty = 1
	}
	p, err := s.products.FindByID(productID)
	if err != nil {
		return err
	}
	if !p.IsActive {
		return errors.New("товар недоступен")
	}
	if p.Stock < qty {
		return errors.New("недостаточно товара на складе")
	}
	return s.cart.Upsert(userID, productID, qty)
}

func (s *CartService) Update(userID, productID int64, qty int) error {
	if qty <= 0 {
		return s.cart.Remove(userID, productID)
	}
	p, err := s.products.FindByID(productID)
	if err != nil {
		return err
	}
	if p.Stock < qty {
		return errors.New("недостаточно товара на складе")
	}
	return s.cart.Update(userID, productID, qty)
}

func (s *CartService) Remove(userID, productID int64) error {
	return s.cart.Remove(userID, productID)
}

func (s *CartService) Count(userID int64) int {
	n, _ := s.cart.Count(userID)
	return n
}

func (s *CartService) Raw() *repository.CartRepository { return s.cart }
