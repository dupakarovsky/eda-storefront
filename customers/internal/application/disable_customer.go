package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// DisableCustomer is a type that can create a update a Cusotmer struct
// communidate with the repository and publish the event to the stream
type DisableCustomer struct {
	customerRepo domain.CustomerRepository
	publisher    *foundationnats.Publisher
	logger       *slog.Logger
}

// NewDisableCustomer constructs a new DisableCustomer type
func NewDisableCustomer(customerRepo domain.CustomerRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *DisableCustomer {
	return &DisableCustomer{
		customerRepo: customerRepo,
		publisher:    publisher,
		logger:       logger,
	}
}

func (uc *DisableCustomer) Execute(ctx context.Context, customerID string) (*domain.Customer, error) {

	var event domain.Event
	var customer *domain.Customer

	err := uc.customerRepo.WithLock(ctx, func(txCtx context.Context) error {

		var err error

		// locate the customer in the repository for update
		customer, err = uc.customerRepo.FindByIDForUpdate(txCtx, customerID)
		if err != nil {
			return fmt.Errorf("locating customer: %w", err)
		}

		// customer found
		// --------------

		// update the customer to Disabled; capture the event
		event, err = customer.Disable()
		if err != nil {
			return fmt.Errorf("disabling customer: %w", err)
		}

		// customer udpated
		// ----------------

		// persist changes to database
		err = uc.customerRepo.Save(txCtx, customer)
		if err != nil {
			return fmt.Errorf("saving customer: %w", err)
		}

		// customer saved
		// --------------

		return nil
	})
	if err != nil {
		return nil, err
	}

	// transaction succeeded.  customer was disabled
	//-----------------------------------------------

	// publishe the Event to the stream
	err = publishDomainEvent(ctx, uc.publisher, uc.logger, event)
	if err != nil {

		//INFO:: If publishDomainEvent fails, the customer remains disabled in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.
		return customer, err
	}

	return customer, nil
}
