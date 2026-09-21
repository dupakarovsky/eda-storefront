// baskets/internal/application/checkout.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// Checkout is a type that can connect to the baskets repository as well as the catalogue.
// extract data from the catalog_stores and catalog_products tables
type Checkout struct {
	basketRepo  domain.BasketRepository
	catalogRepo catalog.Repository
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

func NewCheckout(basketRepo domain.BasketRepository, catalogRepo catalog.Repository, publisher *foundationnats.Publisher, logger *slog.Logger) *Checkout {
	return &Checkout{
		basketRepo:  basketRepo,
		catalogRepo: catalogRepo,
		publisher:   publisher,
		logger:      logger,
	}
}

func (uc *Checkout) Execute(ctx context.Context, basketID string) (*domain.Basket, error) {

	var event domain.Event
	var basket *domain.Basket

	err := uc.basketRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// retrieve the basket
		basket, err = uc.basketRepo.FindByIDForUpdate(txCtx, basketID)
		if err != nil {
			return fmt.Errorf("finding basket: %w", err)
		}

		// we need to check the store associated with each product
		// Items{ {"prod-1","quantity:1"},{"prod-2","quantity:2"} }

		// create a slice of productIDs to consult their stores against the database
		var productIDs []string
		for _, item := range basket.Items {
			productIDs = append(productIDs, item.ProductID)
		}

		// retrive a map of "productID" -> "storeID" for each product id in the list
		storeIDS, err := uc.catalogRepo.FindStoreIDsByProductIDs(ctx, productIDs)
		if err != nil {
			return fmt.Errorf("getting store ids")
		}

		// execute Checkout to emit the event
		event, err = basket.Checkout(storeIDS)
		if err != nil {
			return fmt.Errorf("checking out basket: %w", err)
		}

		// save the basket
		if err := uc.basketRepo.Save(txCtx, basket); err != nil {
			return fmt.Errorf("saving basket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return basket, err
	}

	return basket, nil
}
