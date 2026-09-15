// depot/internal/application/initiate_shopping_list.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// InitiateShoppingList will create a struct that can update the database when the
// Initate() event happens and then publish it to the stream. The Iniitate() will
// modify the status of a shopping list, freezing the addition of any new Stops
type InitiateShoppingList struct {
	shoppingListRepo domain.ShoppingListRepository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

// NewInitiateShoppingList will initialize an InitiateShoppingList struct
func NewInitiateShoppingList(shoppingListRepo domain.ShoppingListRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *InitiateShoppingList {
	return &InitiateShoppingList{
		shoppingListRepo: shoppingListRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

func (uc *InitiateShoppingList) Execute(ctx context.Context, shoppingListID string) (*domain.ShoppingList, error) {

	var event domain.Event
	var shoppingList *domain.ShoppingList

	// initialize the transaciton block
	err := uc.shoppingListRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate the shopping list by it's ID
		shoppingList, err = uc.shoppingListRepo.FindByIDForUpdate(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("finding shopping list: %w", err)
		}

		// emit the Initate() event
		event, err = shoppingList.Initiate()
		if err != nil {
			return fmt.Errorf("initiating shopping list: %w", err)
		}

		// persist the updated shopping list to the database
		if err := uc.shoppingListRepo.Save(txCtx, shoppingList); err != nil {
			return fmt.Errorf("saving shopping list: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// transaction successful
	// -----------------------

	// publish the Initiate Event to the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return shoppingList, err
	}

	return shoppingList, nil
}
