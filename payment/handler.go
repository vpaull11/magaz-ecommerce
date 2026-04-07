package payment

import (
	"encoding/json"
	"io"
	"net/http"

	"magaz/internal/service"
)

type Handler struct {
	svc    *Service
	secret string
}

func NewHandler(svc *Service, secret string) *Handler {
	return &Handler{svc: svc, secret: secret}
}

// POST /charge
func (h *Handler) Charge(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify HMAC signature
	sig := r.Header.Get("X-Signature")
	if !service.VerifySignature(body, sig, h.secret) {
		jsonError(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var req ChargeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.CardNumber) != 16 {
		jsonError(w, "invalid card number", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.Charge(req)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, resp)
}

// GET /status/{txID}
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	txID := r.URL.Query().Get("tx_id")
	if txID == "" {
		jsonError(w, "missing tx_id", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.Status(txID)
	if err != nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, resp)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
