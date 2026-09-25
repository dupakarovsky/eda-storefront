package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

// CreateShoppingList defines an application type responsible for performng the orchestration
// across layers. It'll create new Shopping and receive it's Event, persist it into the
// database and then publish the event into the jetstream
type CreateShoppingList struct {
	shoppingListRepo domain.ShoppingListRepository
	publisher        *foundationnats.Publisher
	logger           *slog.Logger
}

func NewCreateShoppingList(
	shoppingListRepo domain.ShoppingListRepository,
	publisher *foundationnats.Publisher,
	logger *slog.Logger,
) *CreateShoppingList {
	return &CreateShoppingList{
		shoppingListRepo: shoppingListRepo,
		publisher:        publisher,
		logger:           logger,
	}
}

// Execute will create a new domain shoppingList, persist it to the database and publish it's
// event to the stream.
func (uc *CreateShoppingList) Execute(ctx context.Context, id, orderID string) (*domain.ShoppingList, error) {

	// first, verify if there's already a existing shopping list. If exists, return that one
	existing, err := uc.shoppingListRepo.FindByOrderID(ctx, orderID)
	if nil == err {
		uc.logger.Info("shopping list already exists for order, skipping creation", "order_id", orderID)
		return existing, nil
	}
	if !errors.Is(err, domain.ErrShoppingListNotFound) {
		return nil, fmt.Errorf("checking for existing shopping list: %w", err)
	}

	// no shopping list exists yet with the provided order_id
	// ------------------------------------------------------

	// create a new Shopping List
	shoppingList, event, err := domain.NewShoppingList(id, orderID)
	if err != nil {
		return nil, fmt.Errorf("constructing shopping list: %w", err)
	}

	// persist the list to the database
	if err := uc.shoppingListRepo.Save(ctx, shoppingList); err != nil {
		return nil, fmt.Errorf("saving shopping list: %w", err)
	}

	// publish the ShoppingListCreated Event to the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return shoppingList, err
	}

	return shoppingList, nil
}
