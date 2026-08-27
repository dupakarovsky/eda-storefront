// 01-stores/internal/application/increase_product_price.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// IncreaseProductPrice defines a new type that is able to increase the
// Price of a Product
type IncreaseProductPrice struct {
	productRepo domain.ProductRepository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewIncreaseProductPrice instantiates a new IncreaseProductPrice struct
func NewIncreaseProductPrice(productRepo domain.ProductRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *IncreaseProductPrice {
	return &IncreaseProductPrice{
		productRepo: productRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will try to locate an existing product in the repository, increase the price,
// update to the reposiotry and then publish the event to the stream
func (uc *IncreaseProductPrice) Execute(ctx context.Context, productID string, price float64) (*domain.Product, error) {

	// event and store will capture the values returned by the FindByIDForUpdate and RebrandStore
	// methods
	var event domain.Event
	var product *domain.Product

	// WithLock will start a new transaction where the FindByIDForUpdate
	// method will execute, followed by IncreasePrice and Save, to presist the
	// changes to the database.
	err := uc.productRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// try to locate the product
		product, err = uc.productRepo.FindByIDForUpdate(txCtx, productID)
		if err != nil {
			return fmt.Errorf("finding product: %w", err)
		}

		// increase the price to receive the event
		event, err = product.IncreasePrice(price)
		if err != nil {
			return fmt.Errorf("increasing product price: %w", err)
		}

		// persist the changes of the product in the reposiotry
		if err := uc.productRepo.Save(txCtx, product); err != nil {
			return fmt.Errorf("saving product: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
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
