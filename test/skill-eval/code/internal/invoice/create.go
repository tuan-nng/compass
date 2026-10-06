package invoice

import (
	"encoding/json"
	"net/http"
)

// Store saves an invoice and returns it with its ID set.
type Store interface {
	Save(Invoice) (Invoice, error)
}

// Charger charges the customer's default payment method.
type Charger interface {
	Charge(Invoice) error
}

// Handler serves POST /v2/invoices. Every call it receives creates and charges
// a new invoice; repeated keys are caught earlier, by Idempotency.
type Handler struct {
	Store   Store
	Charger Charger
}

// ServeHTTP creates an invoice and charges the customer.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var in Invoice
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	inv, err := h.Store.Save(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.Charger.Charge(inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(inv)
}

// Invoice is the v2 invoice resource.
type Invoice struct {
	ID          string `json:"id"`
	CustomerID  string `json:"customer_id"`
	AmountCents int64  `json:"amount_cents"`
}
