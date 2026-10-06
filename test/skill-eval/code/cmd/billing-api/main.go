package main

import (
	"log"
	"net/http"

	"example.com/billing-api/internal/invoice"
	"example.com/billing-api/internal/payments"
)

func main() {
	h := &invoice.Handler{Store: payments.NewStore(), Charger: payments.NewProvider()}
	// The idempotency middleware runs before the handler, so before the
	// payment call: a repeated key never reaches the payment provider.
	http.Handle("POST /v2/invoices", invoice.Idempotency(h))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
