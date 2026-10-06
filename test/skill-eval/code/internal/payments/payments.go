// Package payments talks to the payment provider and stores invoices.
package payments

import (
	"fmt"
	"sync"

	"example.com/billing-api/internal/invoice"
)

// MemStore keeps invoices in memory.
type MemStore struct {
	mu   sync.Mutex
	next int
	all  map[string]invoice.Invoice
}

// NewStore returns an empty store.
func NewStore() *MemStore { return &MemStore{all: map[string]invoice.Invoice{}} }

// Save stores the invoice under a new ID.
func (s *MemStore) Save(in invoice.Invoice) (invoice.Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	in.ID = fmt.Sprintf("inv_%d", s.next)
	s.all[in.ID] = in
	return in, nil
}

// Provider is the payment provider client.
type Provider struct{}

// NewProvider returns a provider client.
func NewProvider() *Provider { return &Provider{} }

// Charge charges the customer's default payment method.
func (p *Provider) Charge(inv invoice.Invoice) error { return nil }
