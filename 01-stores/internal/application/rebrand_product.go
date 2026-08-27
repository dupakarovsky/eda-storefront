package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// RebrandProduct defines a type that is able to update a Product information
type RebrandProduct struct {
	productRepo domain.ProductRepository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewRebrandProduct instantiates a new  RebrandProduct struct
func NewRebrandProduct(productRepo domain.ProductRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *RebrandProduct {
	return &RebrandProduct{
		productRepo: productRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will first try to locate existing product in the reposiotry, update
// it's name  and/or description, persist the changes to the repository and
// emit and emit an event to the stream
func (uc *RebrandProduct) Execute(ctx context.Context, productID, name, description string) (*domain.Product, error) {

	// event and store will capture the values returned by the FindByIDForUpdate and RebrandStore
	// methods
	var event domain.Event
	var product *domain.Product

	// WithLock will start a new transaction where the FindByIDForUpdate
	// method will execute, followed by Rebrand and Save, to presist the
	// changes to the database.
	err := uc.productRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// try to locate the product in the database
		product, err = uc.productRepo.FindByIDForUpdate(txCtx, productID)
		if err != nil {
			return fmt.Errorf("finding product: %w", err)
		}

		// product found.
		// ----------------------------------------------

		//call Rebrand to emit the event
		event, err = product.Rebrand(name, description)
		if err != nil {
			return fmt.Errorf("rebranding product: %w", err)
		}

		// persist changes to the reposiotry
		if err := uc.productRepo.Save(txCtx, product); err != nil {
			return fmt.Errorf("saving product: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// saved, now publish the event to the stream
	// ------------------------------------------

	//INFO:: If publishDomainEvent fails, the store remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// event published
	err = publishDomainEvent(ctx, uc.publisher, uc.logger, event)
	if err != nil {
		return product, err
	}

	//
	return product, nil
}
