package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// GetCusotmer defines a type that can connect to the customer repository
type GetCustomer struct {
	customerRepo domain.CustomerRepository
}

// NewGetOrder will instantiate a new GetOrder
func NewGetCustomer(customerRepo domain.CustomerRepository) *GetCustomer {
	return &GetCustomer{
		customerRepo: customerRepo,
	}
}

// Execute will simply connect to orders repository and return a orther from the database
func (uc *GetCustomer) Execute(ctx context.Context, customerID string) (*domain.Customer, error) {
	order, err := uc.customerRepo.FindByID(ctx, customerID)
	if err != nil {
		return nil, fmt.Errorf("finding customer: %w", err)
	}

	return order, nil
}
