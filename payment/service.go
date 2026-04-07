package payment

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// Charge simulates card processing.
// Rules: cards starting with 4242 → success; 4000 → fail; others → 70% success.
func (s *Service) Charge(req ChargeRequest) (*ChargeResponse, error) {
	// Artificial processing delay
	time.Sleep(500 * time.Millisecond)

	txID := uuid.New().String()
	last4 := ""
	if len(req.CardNumber) >= 4 {
		last4 = req.CardNumber[len(req.CardNumber)-4:]
	}

	status, message := simulate(req.CardNumber)

	tx := &Transaction{
		ID:        txID,
		OrderID:   req.OrderID,
		Amount:    req.Amount,
		CardLast4: last4,
		Status:    status,
		Message:   message,
	}
	if err := s.repo.Save(tx); err != nil {
		return nil, err
	}

	return &ChargeResponse{TxID: txID, Status: status, Message: message}, nil
}

func (s *Service) Status(txID string) (*StatusResponse, error) {
	tx, err := s.repo.FindByID(txID)
	if err != nil {
		return nil, err
	}
	return &StatusResponse{
		TxID:    tx.ID,
		OrderID: tx.OrderID,
		Amount:  tx.Amount,
		Status:  tx.Status,
		Message: tx.Message,
	}, nil
}

func simulate(cardNumber string) (status, message string) {
	switch {
	case strings.HasPrefix(cardNumber, "4242"):
		return "success", "Оплата прошла успешно"
	case strings.HasPrefix(cardNumber, "4000"):
		return "failed", "Карта отклонена банком"
	case strings.HasPrefix(cardNumber, "5200"):
		return "failed", "Недостаточно средств"
	default:
		// 70% success for other cards
		if (time.Now().UnixNano() % 10) < 7 {
			return "success", "Оплата прошла успешно"
		}
		return "failed", "Ошибка обработки платежа"
	}
}
