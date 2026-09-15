// depot/internal/application/remove_item.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// RemoveItem defines a type that can retrieve data from both the shoppingList Repository
// as well as the catatalog Repository. It'll need to verify that a Store exist in the
// catalog_store table before removing and item from the database
// It'll also then publish Event to the stream
type RemoveItem struct {
	shoppingListRepo domain.ShoppingListRepository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

// NewRemoveItem will initialize a new RemoveItem struct
func NewRemoveItem(shoppingListRepo domain.ShoppingListRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *RemoveItem {
	return &RemoveItem{
		shoppingListRepo: shoppingListRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

// Execute will run the queries to the action on the database and publish the Event to the stream
func (uc *RemoveItem) Execute(ctx context.Context, shoppingListID, storeID, productID string) (*domain.ShoppingList, error) {

	var event domain.Event
	var shoppingList *domain.ShoppingList

	// initialize a transaction lock
	err := uc.shoppingListRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate a shopping list by it's ID. This is the list to be updated
		shoppingList, err = uc.shoppingListRepo.FindByIDForUpdate(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("finding shopping list: %w", err)
		}

		// call RemoveItem to remove an Item from the list and emit the Event
		// (An Item belongs to a Store Stop in the list)
		event, err = shoppingList.RemoveItem(storeID, productID)
		if err != nil {
			return fmt.Errorf("removing item: %w", err)
		}

		// persist the changes on the shopping list to the database
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

	// publish the RemoveItem Event to the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return shoppingList, err
	}

	// return the updated list
	return shoppingList, nil
}
