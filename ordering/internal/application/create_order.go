package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// CreateOrder defines type use case that can connect to the orders repostiory and
// read/write to the ordering database
type CreateOrder struct {
	orderRepo   domain.OrderRepository
	depotClient domain.DepotClient
	publisher   *foundationnats.Publisher
	logger      *slog.Logger
}

// NewCreateOrder instantiates a new NewCreateOrder Use Case type
func NewCreateOrder(orderRepo domain.OrderRepository, depoClient domain.DepotClient, publisher *foundationnats.Publisher, logger *slog.Logger) *CreateOrder {
	return &CreateOrder{
		orderRepo:   orderRepo,
		depotClient: depoClient,
		publisher:   publisher,
		logger:      logger,
	}
}

// Execute will be called by the baskets.Handler which will run when a new event (CheckedOutBasket) is published
// When that event is published we'll consturct an Order with the same ID as the basket.
// This is needed so handler doesn't construct a new Order more than once during a re-try
func (uc *CreateOrder) Execute(ctx context.Context, id string, items []domain.Item) (*domain.Order, error) {

	// first check if there's already an Order associated with with this ID.
	existing, err := uc.orderRepo.FindByID(ctx, id)
	// If there is, than there are two possible outcomes:

	if nil == err {
		// A) a shopping list is already linked to the order:
		// it was created successfuly in `depot` and all Items were added successfuly.
		// So this Execute call is a retry from publishDomainEvent trying to publish the
		// OrderCreated Event and we don't need to create the another order here.
		// Simply return the existing order.
		if existing.ShoppingListID != "" {
			uc.logger.Info("order already fully orchestrated, skipping", "order_id", id)
			return existing, nil
		}

		// B) a shopping list exists but is not yet completed. We need to add make the
		// rpc call to Add the Items and then link the list to this existing order.
		uc.logger.Info("order already exists, skipping creation", "order_id", id)
		return uc.orchestrateShoppingList(ctx, existing)
	}

	// some other unkown error happened here
	if !errors.Is(err, domain.ErrOrderNotFound) {
		return nil, fmt.Errorf("checking for existing order: %w", err)
	}

	// from here the no Order was found (ErrOrderNotFound);
	// proceed into creating a new order
	//-----------------------------------------------------

	// create a new Order form the order id and items received from the subscriber
	order, event, err := domain.NewOrder(id, items)
	if err != nil {
		return nil, fmt.Errorf("constructing order: %w", err)
	}

	// use the repository Save method to save the order to the database
	if err := uc.orderRepo.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("saving order: %w", err)
	}

	// publish the order into the stream
	if err := publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {

		//INFO:: If publishDomainEvent fails, the order remains created in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.
		return order, err
	}

	// order was created and we need to communicate with depot via rpc to command it to create
	// a shopping list with our Order.ID
	// ---------------------------------------------------------------------------------------

	// make the orchestration
	return uc.orchestrateShoppingList(ctx, order)
}

//-----------------------------------------------------------------------------

// orchestrateShoppingList will communicate with the depot service via a rpc and
// request the creation of shopping list given our order id. It'll then make a sequence
// of rpc requests to add each item in the order to the shopping list. Finally, it'll
// link the shopping list id to the order
func (uc *CreateOrder) orchestrateShoppingList(ctx context.Context, order *domain.Order) (*domain.Order, error) {

	// INFO: on every failure path here need to return the order to the caller (event on failure) since
	// the order is already saved to the database and the OrderCreated event was published to the stream.

	// make a rpc call to the depot service to create a new shopping list and recieve a
	// shopping list ID
	shoppingListID, err := uc.depotClient.CreateShoppingList(ctx, order.ID)
	if err != nil {
		return order, fmt.Errorf("creating shopping list: %w", err)
	}

	// shopping list returned by the `depot` service.
	// Now make another rpc request to check the quantites of each item
	// in this list
	// -----------------------------------------------------------------

	existingQuantities, err := uc.depotClient.GetShoppingList(ctx, shoppingListID)
	if err != nil {
		return order, fmt.Errorf("checking existing shopping list contents: %w", err)
	}

	// now make a sequence or rpc requests to add items to this shopping list.
	for _, item := range order.Items {

		// we'll only call AddItem to match remaining quantities
		remaining := item.Quantity - existingQuantities[item.ProductID]

		for range remaining {
			if err := uc.depotClient.AddItem(ctx, shoppingListID, item.StoreID, item.ProductID); err != nil {
				return order, fmt.Errorf("adding item %q to shopping list %q: %w", item.ProductID, shoppingListID, err)
			}
		}
	}

	// all items where added to the list. The list is completed in depot
	// -----------------------------------------------------------------

	// link the order to the list
	if err := order.LinkShoppingList(shoppingListID); err != nil {
		return order, fmt.Errorf("linking shopping list: %w", err)
	}

	// save the order in the database
	if err := uc.orderRepo.Save(ctx, order); err != nil {
		return order, fmt.Errorf("saving order after linking shopping list: %w", err)
	}

	// return the order
	return order, nil
}
