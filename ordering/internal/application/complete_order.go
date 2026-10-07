package application

import (
	"context"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// CompleteOrder defines a type that can communicate with th order repository in order
// to complete an order and issue and order readied event
type CompleteOrder struct {
	orderRepo domain.OrderRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewCompleteOrder will instantiate the new use case from main
func NewCompleteOrder(orderRepo domain.OrderRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *CompleteOrder {
	return &CompleteOrder{
		orderRepo: orderRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute will need first to lookup an order in the repository by a shopping list ID.
// when found the events needs to have it's status checked to see if it's in "created",
// in which case we need to update the status to "readied" and emit the event, or if it's
// aready "readied" we simply return that order.
func (uc *CompleteOrder) Execute(ctx context.Context, shoppingListID string) (*domain.Order, error) {

	var event domain.Event  // receives any concrete Event
	var order *domain.Order // will receive the order retrieved

	// boolean variable to tracks whether the found order was been already readied
	alreadyReadied := false

	// initilaize a transaction that creates an order
	err := uc.orderRepo.WithLock(ctx, func(txCtx context.Context) error {

		// use the transaction object inside context ad call the repository method to
		// locate an order associated with this ID
		foundOrder, err := uc.orderRepo.FindByShoppingListID(txCtx, shoppingListID)
		if err != nil {
			return fmt.Errorf("locating order: %w", err)
		}

		// retrieve the record from the orders database associated with this shoppingListID
		// in order to updated it's status.
		order, err = uc.orderRepo.FindByIDForUpdate(txCtx, foundOrder.ID)
		if err != nil {
			return fmt.Errorf("finding order by id: %w", err)
		}

		// check the status of the found order it's in OrderReadiedStatus.
		// if it's already readied, then we return early since invoking MarkReadied
		// next would cause an error (ErrOrderAlreadyReadied).
		// Here, the transaction commits successfuly but without any updates.
		if order.Status == domain.OrderReadiedStatus {
			alreadyReadied = true
			return nil
		}

		// update the status of the order and capture the event for publication
		// to the stream
		event, err = order.MarkReadied()
		if err != nil {
			return fmt.Errorf("marking order readied: %w", err)
		}

		// persist the updated order to the database
		if err := uc.orderRepo.Save(txCtx, order); err != nil {
			return fmt.Errorf("saving order: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// passed here transaction succeeded and the orders status has been
	// updated to OrderReadiedStatusk
	// ----------------------------------------------------------------

	// now check if the order retrieve was already in the Ready status.
	// if so, we don't need to publish the event again. just return the order
	//
	if alreadyReadied {
		uc.logger.Info("order already readied, skipping", "order_id", order.ID, "shopping_list_id", shoppingListID)
		return order, nil
	}

	//INFO:: If publishDomainEvent fails, the order remains readied in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publish the event now
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return order, err
	}

	return order, nil

}
