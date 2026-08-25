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

	// try to locate the product in the database
	product, err := uc.productRepo.FindByIDForUpdate(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("finding product: %w", err)
	}

	// product found.
	// ----------------------------------------------

	//call Rebrand to emit the event
	event, err := product.Rebrand(name, description)
	if err != nil {
		return nil, fmt.Errorf("rebranding product: %w", err)
	}

	// persist changes to the reposiotry
	if err := uc.productRepo.Save(ctx, product); err != nil {
		return nil, fmt.Errorf("saving product: %w", err)
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
