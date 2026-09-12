// baskets/internal/application/add_item.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// AddItem defines an application type responsible for perform the orchestration
// across layers. It'll create new Basket and receive it's Event, persist it into the
// database and then publish the event into the jetstream
type AddItem struct {
	basketRepo  domain.BasketRepository
	catalogRepo catalog.Repository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewAddItem instantiates a new AddItem USE CASE handler
func NewAddItem(basketRepo domain.BasketRepository, catalogRepo catalog.Repository, publisher *foundationnats.Publisher, logger *slog.Logger) *AddItem {
	return &AddItem{
		basketRepo:  basketRepo,
		catalogRepo: catalogRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will create first ...
func (uc *AddItem) Execute(ctx context.Context, basketID, productID string) (*domain.Basket, error) {

	var event domain.Event
	var basket *domain.Basket

	// begin a transaction.
	err := uc.basketRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate a product we need to add to the basket by it's ID in the catalog_products table
		if _, err = uc.catalogRepo.FindProductByID(txCtx, productID); err != nil {
			return fmt.Errorf("validating product: %w", err)
		}

		// past here product was found (confimed the product exists)
		// ---------------------------------------------------------

		// will locate the basket the product will be added to
		basket, err = uc.basketRepo.FindByIDForUpdate(txCtx, basketID)
		if err != nil {
			return fmt.Errorf("finding basket: %w", err)
		}

		// add the product to the basket and receive the event
		event, err = basket.AddItem(productID)
		if err != nil {
			return fmt.Errorf("adding item: %w", err)
		}

		// save the basket to the database
		if err := uc.basketRepo.Save(txCtx, basket); err != nil {
			return fmt.Errorf("saving basket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// past here the transaction has completed
	// ---------------------------------------

	// publish the event to the stream

	//INFO:: If publishDomainEvent fails, the item remains added to the basket and save to the db.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return basket, err
	}

	return basket, nil
}
