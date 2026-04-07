package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PaymentClient calls the payment microservice.
type PaymentClient struct {
	baseURL string
	secret  string
	client  *http.Client
}

func NewPaymentClient(baseURL, secret string) *PaymentClient {
	return &PaymentClient{
		baseURL: baseURL,
		secret:  secret,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type ChargeRequest struct {
	OrderID    int64   `json:"order_id"`
	Amount     float64 `json:"amount"`
	CardNumber string  `json:"card_number"`
	Expiry     string  `json:"expiry"`
	CVV        string  `json:"cvv"`
}

type ChargeResponse struct {
	TxID    string `json:"tx_id"`
	Status  string `json:"status"`  // "success" | "failed"
	Message string `json:"message"`
}

func (c *PaymentClient) Charge(req ChargeRequest) (*ChargeResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/charge", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Signature", sign(body, c.secret))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("payment service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("payment service error %d: %s", resp.StatusCode, string(b))
	}

	var result ChargeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks the HMAC signature of an incoming request body.
func VerifySignature(body []byte, signature, secret string) bool {
	expected := sign(body, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}
