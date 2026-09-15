package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// AddItem defines a type that can retrieve data from both the shoppingList Repository
// as well as the catatalog Repository. It'll need to verify that a Store exist in the
// catalog_store table before persisting data ShoppingList data into the database.
// It'll then publish Event to the stream
type AddItem struct {
	shoppingListRepo domain.ShoppingListRepository
	catalogRepo      catalog.Repository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

// NewAddItem will instantiate a new AddItem struct
func NewAddItem(shoppingListRepo domain.ShoppingListRepository, catalogRepo catalog.Repository, publisher *foundationnats.Publisher, logger *slog.Logger) *AddItem {
	return &AddItem{
		shoppingListRepo: shoppingListRepo,
		catalogRepo:      catalogRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

func (uc *AddItem) Execute(ctx context.Context, shoppingListID, storeID, productID string) (*domain.ShoppingList, error) {

	// guard against empty store id
	if storeID == "" {
		return nil, domain.ErrStoreIDIsBlank
	}

	// guard agains emtpy product id
	if productID == "" {
		return nil, domain.ErrProductIDIsBlank
	}

	var event domain.Event
	var shoppingList *domain.ShoppingList

	// start a transaction lock to look up and persist data
	err := uc.shoppingListRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// verify if the store exist for the provided id
		store, err := uc.catalogRepo.FindStoreByID(txCtx, storeID)
		if err != nil {
			return fmt.Errorf("validating store: %w", err)
		}

		// store exists
		// ------------

		// verify if the product exists under this id
		if _, err = uc.catalogRepo.FindProductByID(txCtx, productID); err != nil {
			return fmt.Errorf("validating product: %w", err)
		}

		// product exists
		// --------------

		// locate an existing shopping list in the database for update
		shoppingList, err = uc.shoppingListRepo.FindByIDForUpdate(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("finding shopping list: %w", err)
		}

		// add a new item to the this shopping list (need to provide the data of the store)
		event, err = shoppingList.AddItem(store.ID, store.Name, store.Location, productID)
		if err != nil {
			return fmt.Errorf("adding item: %w", err)
		}

		// save the shopping list to the repository
		if err := uc.shoppingListRepo.Save(txCtx, shoppingList); err != nil {
			return fmt.Errorf("saving shopping list: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// transaction successful
	// ----------------------

	// publish the AddItem event to the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return shoppingList, err
	}

	// return the shopping list
	return shoppingList, nil
}
