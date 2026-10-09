package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// EnableCustomer is a type that can create a update a Customer struct
// communidate with the repository and publish the event to the stream
type EnableCustomer struct {
	customerRepo domain.CustomerRepository
	publisher    *foundationnats.Publisher
	logger       *slog.Logger
}

// NewEnableCustomer constructs a new EnableCustomer type
func NewEnableCustomer(customerRepo domain.CustomerRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *EnableCustomer {
	return &EnableCustomer{
		customerRepo: customerRepo,
		publisher:    publisher,
		logger:       logger,
	}
}

func (uc *EnableCustomer) Execute(ctx context.Context, id string) (*domain.Customer, error) {

	var event domain.Event
	var customer *domain.Customer

	err := uc.customerRepo.WithLock(ctx, func(txCtx context.Context) error {

		var err error

		// locate the customer in the repository for update
		customer, err = uc.customerRepo.FindByIDForUpdate(txCtx, id)
		if err != nil {
			return fmt.Errorf("locating customer: %w", err)
		}

		// customer found
		// --------------

		// update the customer to Disabled; capture the event
		event, err = customer.Enable()
		if err != nil {
			return fmt.Errorf("enabling customer: %w", err)
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

	// published the Event to the stream
	err = publishDomainEvent(ctx, uc.publisher, uc.logger, event)
	if err != nil {

		//INFO:: If publishDomainEvent fails, the customer remains enabled in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.
		return customer, err
	}

	return customer, nil
}
