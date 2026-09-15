// depot/internal/application/complete_shopping_list.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// CompleteShoppingList will define a type that can update the shopping list
// and publish the Complete event to the stream
type CompleteShoppingList struct {
	shoppingListRepo domain.ShoppingListRepository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

func NewCompleteShoppingList(shoppingListRepo domain.ShoppingListRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CompleteShoppingList {
	return &CompleteShoppingList{
		shoppingListRepo: shoppingListRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

func (uc *CompleteShoppingList) Execute(ctx context.Context, shoppingListID string) (*domain.ShoppingList, error) {

	var event domain.Event
	var shoppingList *domain.ShoppingList

	err := uc.shoppingListRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		shoppingList, err = uc.shoppingListRepo.FindByIDForUpdate(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("finding shopping list: %w", err)
		}

		event, err = shoppingList.Complete()
		if err != nil {
			return fmt.Errorf("completing shopping list: %w", err)
		}

		if err := uc.shoppingListRepo.Save(txCtx, shoppingList); err != nil {
			return fmt.Errorf("saving shopping list: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return shoppingList, err
	}

	return shoppingList, nil
}
