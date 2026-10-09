package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// RegisterCustomer is a type that can create a Customer type, communidate
// with the repository and publish the event to the stream
type RegisterCustomer struct {
	customerRepo domain.CustomerRepository
	publisher    *foundationnats.Publisher
	logger       *slog.Logger
}

// NewRegisterCustomer constructs a new RegisterCustomer type
func NewRegisterCustomer(customerRepo domain.CustomerRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *RegisterCustomer {
	return &RegisterCustomer{
		customerRepo: customerRepo,
		publisher:    publisher,
		logger:       logger,
	}
}

func (uc *RegisterCustomer) Execute(ctx context.Context, id, name, smsNumber string) (*domain.Customer, error) {

	// generate a new Customer from the parameters
	customer, event, err := domain.NewCustomer(id, name, smsNumber)
	if err != nil {
		return nil, fmt.Errorf("constructing customer: %w", err)
	}

	// customer created and event captured
	// -----------------------------------

	// save to the repository
	err = uc.customerRepo.Save(ctx, customer)
	if err != nil {
		return nil, fmt.Errorf("saving cusotmer: %w", err)
	}

	// customer saved; publish the event
	// --------------------------------

	//INFO:: If publishDomainEvent fails, the order remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.
	err = publishDomainEvent(ctx, uc.publisher, uc.logger, event)
	if err != nil {
		return customer, err
	}

	// publishing succeeded
	// ---------------------

	// return the created customer
	return customer, nil

}
