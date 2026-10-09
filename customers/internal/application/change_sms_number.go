package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// ChangeSmsNumber is a type that can create a update a Custmer struct
// communidate with the repository and publish the event to the stream
type ChangeSmsNumber struct {
	customerRepo domain.CustomerRepository
	publisher    *foundationnats.Publisher
	logger       *slog.Logger
}

// NewRegisterCustomer constructs a new RegisterCustomer type
func NewChangeSmsNumber(customerRepo domain.CustomerRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *ChangeSmsNumber {
	return &ChangeSmsNumber{
		customerRepo: customerRepo,
		publisher:    publisher,
		logger:       logger,
	}
}

func (uc *ChangeSmsNumber) Execute(ctx context.Context, customerID, smsNumber string) (*domain.Customer, error) {

	var event domain.Event
	var customer *domain.Customer

	// initialize a transaction: locate the customer, change the sms number and
	// persist changes
	err := uc.customerRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate the customer in the repository for update
		customer, err = uc.customerRepo.FindByIDForUpdate(txCtx, customerID)
		if err != nil {
			return fmt.Errorf("finding customer: %w", err)
		}

		// customer found
		// --------------

		// Update the customer number and capture the event
		event, err = customer.ChangeSmsNumber(smsNumber)
		if err != nil {
			return fmt.Errorf("changing sms number: %w", err)
		}

		// save to repository
		err = uc.customerRepo.Save(txCtx, customer)
		if err != nil {
			return fmt.Errorf("saving customer: %w", err)
		}

		// success saving
		// --------------

		return nil
	})
	if err != nil {
		return nil, err
	}

	// transaction successful. customer sms updated
	// --------------------------------------------

	// publish the event
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		//INFO:: If publishDomainEvent fails, the customer sms number remains changed in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.
		return customer, err
	}

	return customer, nil
}
