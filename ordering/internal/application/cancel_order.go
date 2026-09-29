package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// CancelOrder defines a type that can communicate with the depot service
// using a client (any concrete type that knowns to the send requests via
// rpc -- in this case the depotClient.Client type), can update the order
// in the orders database and can publish events to the stream
type CancelOrder struct {
	orderRepo   domain.OrderRepository
	depotClient domain.DepotClient
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewCancelOrder instantiates a new CancelOrder type
func NewCancelOrder(orderRepo domain.OrderRepository, depotClient domain.DepotClient, publisher *foundationnats.Publisher, logger *slog.Logger) *CancelOrder {
	return &CancelOrder{
		orderRepo:   orderRepo,
		depotClient: depotClient,
		publisher:   publisher,
		logger:      logger,
	}
}

func (uc *CancelOrder) Execute(ctx context.Context, orderID string) (*domain.Order, error) {

	var event domain.Event
	var order *domain.Order

	// initialize a transaction block
	err := uc.orderRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate the order in the database that will need to be canceled
		order, err = uc.orderRepo.FindByIDForUpdate(txCtx, orderID)
		if err != nil {
			return fmt.Errorf("finding order: %w", err)
		}

		// call the Cancel method and capture the Event
		event, err = order.Cancel()
		if err != nil {
			return fmt.Errorf("canceling order: %w", err)
		}

		// order is now in Canceled state
		// ------------------------------

		// save back to the database
		if err := uc.orderRepo.Save(ctx, order); err != nil {
			return fmt.Errorf("saving order: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// passed here the order was successfuly updated the database and is
	// not marked as status Canceled.
	// -----------------------------------------------------------------

	// Since the order has been canceled, we'll now need to cancel the ShoppingList
	// by communicating with the depot service and requesting a shopping list
	// cancellation

	// We can only try to cancel orders tha have a shopping list linked to it
	if order.ShoppingListID != "" {
		// call the service via rpc and capture any returned errors
		if err := uc.depotClient.CancelShoppingList(ctx, order.ShoppingListID); err != nil {
			// if there was an error, the depot RPC Server returned a mapError type
			// (connect.CodeNotFound, CodeInvalidArgument or CodeInternal),
			var connectErr *connect.Error
			// if the code was CodeInvalidArgument, then we tried to Cancel an order that was
			// already past it's cancelation point.
			if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeInvalidArgument {
				uc.logger.Info("shopping list already in a terminal state, proceeding with order cancellation",
					"order_id", order.ID,
					"shopping_list_id", order.ShoppingListID,
				)
				// some other type (either CodeNotFound or CodeInternal)
			} else {
				return order, fmt.Errorf("canceling shopping list: %w", err)
			}
		}
	}

	//INFO:: If publishDomainEvent fails, the order remains canceled in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	// publish the CancelOrder event to the order's jetstream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return order, err
	}

	return order, nil
}
