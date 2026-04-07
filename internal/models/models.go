package models

import (
	"fmt"
	"time"
)

// ─── Roles ───────────────────────────────────────────────────────────────────

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// ─── Order statuses ──────────────────────────────────────────────────────────

type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusPaid      OrderStatus = "paid"
	StatusShipped   OrderStatus = "shipped"
	StatusDone      OrderStatus = "done"
	StatusCancelled OrderStatus = "cancelled"
)

func (s OrderStatus) Label() string {
	switch s {
	case StatusPending:
		return "Ожидает оплаты"
	case StatusPaid:
		return "Оплачен"
	case StatusShipped:
		return "Отправлен"
	case StatusDone:
		return "Выполнен"
	case StatusCancelled:
		return "Отменён"
	}
	return string(s)
}

// ─── User ─────────────────────────────────────────────────────────────────────

type User struct {
	ID           int64      `db:"id"`
	Email        string     `db:"email"`
	PasswordHash string     `db:"password_hash"`
	Name         string     `db:"name"`
	Role         Role       `db:"role"`
	ResetToken   *string    `db:"reset_token"`
	ResetExpires *time.Time `db:"reset_expires"`
	CreatedAt    time.Time  `db:"created_at"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// ─── Category ────────────────────────────────────────────────────────────────

type Category struct {
	ID             int64      `db:"id"`
	Name           string     `db:"name"`
	Slug           string     `db:"slug"`
	ParentID       *int64     `db:"parent_id"`
	SortOrder      int        `db:"sort_order"`
	SeoTitle       string     `db:"seo_title"`
	SeoDescription string     `db:"seo_description"`
	Children       []*Category `db:"-"` // populated in-memory
}

// SiteSettings is a flat key→value map loaded from the site_settings table.
type SiteSettings map[string]string

func (s SiteSettings) Get(key, fallback string) string {
	if v, ok := s[key]; ok && v != "" {
		return v
	}
	return fallback
}

// ─── Attributes ───────────────────────────────────────────────────────────────

type AttrDef struct {
	ID         int64  `db:"id"`
	CategoryID int64  `db:"category_id"`
	Name       string `db:"name"`
	Slug       string `db:"slug"`
	ValueType  string `db:"value_type"` // "string" | "number"
	SortOrder  int    `db:"sort_order"`
}

type AttrValue struct {
	ID         int64    `db:"id"`
	ProductID  int64    `db:"product_id"`
	AttrDefID  int64    `db:"attr_def_id"`
	ValueStr   *string  `db:"value_str"`
	ValueNum   *float64 `db:"value_num"`
	Def        *AttrDef `db:"-"`
}

func (av *AttrValue) DisplayValue() string {
	if av.ValueStr != nil {
		return *av.ValueStr
	}
	if av.ValueNum != nil {
		return fmt.Sprintf("%g", *av.ValueNum)
	}
	return ""
}

// ─── Product ──────────────────────────────────────────────────────────────────

type Product struct {
	ID             int64      `db:"id" json:"id"`
	Name           string     `db:"name" json:"name"`
	Description    string     `db:"description" json:"description"`
	Price          float64    `db:"price" json:"price"`
	Stock          int        `db:"stock" json:"stock"`
	ImageURL       string     `db:"image_url" json:"image_url"`
	CategoryID     int64      `db:"category_id" json:"category_id"`
	SeoTitle       string     `db:"seo_title" json:"-"`
	SeoDescription string     `db:"seo_description" json:"-"`
	CategoryName string     `db:"category_name" db_ignore:"true" json:"category_name,omitempty"`
	IsActive     bool       `db:"is_active" json:"is_active"`
	RatingAvg    float64    `db:"rating_avg" json:"rating_avg"`
	ReviewCount  int        `db:"review_count" json:"review_count"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	Attrs        []AttrValue `db:"-" json:"attrs,omitempty"`
}

// ─── Address ─────────────────────────────────────────────────────────────────

type Address struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Label     string    `db:"label"`
	City      string    `db:"city"`
	Street    string    `db:"street"`
	Zip       string    `db:"zip"`
	IsDefault bool      `db:"is_default"`
	CreatedAt time.Time `db:"created_at"`
}

func (a *Address) Full() string {
	return a.Zip + ", г. " + a.City + ", " + a.Street
}

// ─── Cart ─────────────────────────────────────────────────────────────────────

type CartItem struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	ProductID int64     `db:"product_id"`
	Quantity  int       `db:"quantity"`
	AddedAt   time.Time `db:"added_at"`
	Product   *Product  `db:"-"`
}

func (c *CartItem) Subtotal() float64 {
	if c.Product == nil {
		return 0
	}
	return c.Product.Price * float64(c.Quantity)
}

// ─── Order ────────────────────────────────────────────────────────────────────

type Order struct {
	ID          int64       `db:"id" json:"id"`
	UserID      int64       `db:"user_id" json:"user_id"`
	AddressID   *int64      `db:"address_id" json:"address_id,omitempty"`
	Status      OrderStatus `db:"status" json:"status"`
	TotalAmount float64     `db:"total_amount" json:"total_amount"`
	PaymentTxID *string     `db:"payment_tx_id" json:"payment_tx_id,omitempty"`
	CreatedAt   time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time   `db:"updated_at" json:"updated_at"`
	Items       []OrderItem `db:"-" json:"items,omitempty"`
	Address     *Address    `db:"-" json:"address,omitempty"`
}

// ─── Features ─────────────────────────────────────────────────────────────────

type WishlistItem struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	ProductID int64     `db:"product_id" json:"product_id"`
	AddedAt   time.Time `db:"added_at" json:"added_at"`
	Product   *Product  `db_ignore:"true" json:"product,omitempty"`
}

type Review struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	ProductID int64     `db:"product_id" json:"product_id"`
	Rating    int       `db:"rating" json:"rating"`
	Comment   string    `db:"comment" json:"comment"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UserName  string    `db_ignore:"true" json:"user_name,omitempty"` // For display
}

type OrderItem struct {
	ID            int64   `db:"id"`
	OrderID       int64   `db:"order_id"`
	ProductID     int64   `db:"product_id"`
	ProductName   string  `db:"product_name"`
	Quantity      int     `db:"quantity"`
	PriceSnapshot float64 `db:"price_snapshot"`
}

func (i *OrderItem) Subtotal() float64 { return i.PriceSnapshot * float64(i.Quantity) }
