package application

import (
	"context"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// AuthorizeCustomer is a type that can authroizes a Customer
// communidate with the repository and publish the event to the stream
type AuthorizeCustomer struct {
	customerRepo domain.CustomerRepository
}

func NewAuthorizeCustomer(customerRepo domain.CustomerRepository) *AuthorizeCustomer {
	return &AuthorizeCustomer{
		customerRepo: customerRepo,
	}
}

// Authorize does not mutates a Customer so we don't need to execute it
// inside a transaction block
func (uc *AuthorizeCustomer) Execute(ctx context.Context, customerID string) error {

	// Locate a customer in the database
	customer, err := uc.customerRepo.FindByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("finding customer: %w", err)
	}

	// execute Authorize on this customer
	if err := customer.Authorize(); err != nil {
		return fmt.Errorf("authorizing customer: %w", err)
	}

	return nil
}
