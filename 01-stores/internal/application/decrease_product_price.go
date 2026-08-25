// 01-stores/internal/application/decrease_product_price.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// DecreaseProductPrice defines a struct that is able to decrease a Product's
// price
type DecreaseProductPrice struct {
	productRepo domain.ProductRepository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewDecreaseProductPrice instantes a new DecreaseProductPrice struct
func NewDecreaseProductPrice(productRepo domain.ProductRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *DecreaseProductPrice {
	return &DecreaseProductPrice{
		productRepo: productRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will first locate a product in the repository, decrease it's price and update
// the changes and then publish the event to the stream.
func (uc *DecreaseProductPrice) Execute(ctx context.Context, productID string, price float64) (*domain.Product, error) {

	// locate the product in the repository
	product, err := uc.productRepo.FindByIDForUpdate(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("finding product: %w", err)
	}

	// product located
	// ----------------

	// decrease the product's price and receive the event
	event, err := product.DecreasePrice(price)
	if err != nil {
		return nil, fmt.Errorf("decreasing product price: %w", err)
	}

	// persist the changes to the database
	if err := uc.productRepo.Save(ctx, product); err != nil {
		return nil, fmt.Errorf("saving product: %w", err)
	}

	//INFO:: If publishDomainEvent fails, the product remains updated in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publish the event to the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return product, err
	}

	return product, nil
}
