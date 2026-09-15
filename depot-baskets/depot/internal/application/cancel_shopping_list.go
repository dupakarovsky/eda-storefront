// depot/internal/application/cancel_shopping_list.go
package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

type CancelShoppingList struct {
	shoppingListRepo domain.ShoppingListRepository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

func NewCancelShoppingList(shoppingListRepo domain.ShoppingListRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CancelShoppingList {
	return &CancelShoppingList{
		shoppingListRepo: shoppingListRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

func (uc *CancelShoppingList) Execute(ctx context.Context, shoppingListID string) (*domain.ShoppingList, error) {

	var event domain.Event
	var shoppingList *domain.ShoppingList

	err := uc.shoppingListRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		shoppingList, err = uc.shoppingListRepo.FindByIDForUpdate(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("finding shopping list: %w", err)
		}

		event, err = shoppingList.Cancel()
		if err != nil {
			return fmt.Errorf("canceling shopping list: %w", err)
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
