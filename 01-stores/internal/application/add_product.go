package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// AddProduct defines a struct that can connect to the repository and publishe events
// relatated to adding a product
type AddProduct struct {
	productRepo domain.ProductRepository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewAddProduct instnatiates a new AddProduct struct
func NewAddProduct(productRepo domain.ProductRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *AddProduct {
	return &AddProduct{
		productRepo: productRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will create a create a new product, persist it on the database then publish
// an event on the stream
func (uc *AddProduct) Execute(ctx context.Context, id, storeID, name, description, sku string, price float64) (*domain.Product, error) {

	// instantiate a new product
	product, event, err := domain.NewProduct(id, storeID, name, description, sku, price)
	if err != nil {
		return nil, fmt.Errorf("constructing product: %w", err)
	}

	// save it to the reposiotry
	if err := uc.productRepo.Save(ctx, product); err != nil {
		return nil, fmt.Errorf("saving product: %w", err)
	}

	//INFO:: If publishDomainEvent fails, the product remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publish the event
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return product, err
	}

	return product, nil
}
