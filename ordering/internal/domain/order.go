package domain

import (
	"errors"
	"time"
)

var (
	ErrOrderAlreadyReadied       = errors.New("the order is already readied")
	ErrOrderAlreadyCanceled      = errors.New("the order is already canceled")
	ErrOrderNotYetReady          = errors.New("the order is not ready to be readied")
	ErrOrderIsEmpty              = errors.New("the order has no items")
	ErrShoppingListIDIsBlank     = errors.New("the shopping list id cannot be blank")
	ErrShoppingListAlreadyLinked = errors.New("the order already has a linked shopping list")
	ErrShoppingListNotLinked     = errors.New("the order has no linked shopping list")
)

// Item defines an entry in an order with it's associated
// product ID, store ID and quanitty.Order's Item needs the
// StoreID as it needs to forward it to depot.AddItem per unit.
// depot groups by Items by Stops (storeID) and "ordering" is the
// one that provides it
type Item struct {
	ProductID string
	StoreID   string
	Quantity  int
}

// Order defines an order made by customer in the system when an
// BaskeCheckout happens. ShoppingListID stores the id value returned
// by depot.CreateShoppingList
type Order struct {
	ID             string
	Status         OrderStatus
	Items          []Item
	ShoppingListID string
}

// NewOrder instantiates a new Order struct from the data it receives
func NewOrder(id string, items []Item) (*Order, Event, error) {

	// guard agains an empty list of Items
	if len(items) == 0 {
		return nil, nil, ErrOrderIsEmpty
	}

	// create a new Order instance with Status OrderCreatedStatus
	o := &Order{
		ID:     id,
		Status: OrderCreatedStatus,
		Items:  items,
	}

	// emit the OrderCreated Event type
	event := OrderCreated{
		eventBase: eventBase{aggregateID: id, occurredAt: time.Now()},
		Items:     items,
	}
	return o, event, nil
}

// LinkShoppingList will receive a shopping list id, check if the id has
// already been associated with with order, storing it in the shoppingListID
// field when its not
func (o *Order) LinkShoppingList(shoppingListID string) error {

	// guard against an blank shopping list id
	if shoppingListID == "" {
		return ErrShoppingListIDIsBlank
	}

	// the shoppingListID field for this order should be empty
	// at this stage
	if o.ShoppingListID != "" {
		return ErrShoppingListAlreadyLinked
	}

	// passed the checks
	// -----------------

	// passed here it's ok to link
	o.ShoppingListID = shoppingListID

	return nil
}

// MarkReadied will update the Order's status from "created" to "readied"
// after some checks.
func (o *Order) MarkReadied() (Event, error) {

	// error if the order is already in "readied" status
	if o.Status == OrderReadiedStatus {
		return nil, ErrOrderAlreadyReadied
	}

	// error if the order is in "canceled" status
	if o.Status == OrderCanceledStatus {
		return nil, ErrOrderAlreadyCanceled
	}

	// don't proceed without a linked ShoppingListID
	if o.ShoppingListID == "" {
		return nil, ErrShoppingListNotLinked
	}

	// passed the checks
	// -----------------

	// updates the status
	o.Status = OrderReadiedStatus

	// emit an OrderReadied Event type
	return OrderReadied{
		eventBase: eventBase{aggregateID: o.ID, occurredAt: time.Now()},
	}, nil
}

// Cancel will update the Order's status from "created" to "canceled"
// after some checks.
func (o *Order) Cancel() (Event, error) {

	// error if order is readied (cannot be canceled anymore)
	if o.Status == OrderReadiedStatus {
		return nil, ErrOrderAlreadyReadied
	}

	// error if the order has already been canceled
	if o.Status == OrderCanceledStatus {
		return nil, ErrOrderAlreadyCanceled
	}

	// passed the checks; proceed to cancel
	// ------------------------------------

	// updates the status to canceled
	o.Status = OrderCanceledStatus

	// emit the OrderCanceled Event type
	return OrderCanceled{
		eventBase: eventBase{aggregateID: o.ID, occurredAt: time.Now()},
	}, nil
}

// ReconstituteOrder will take data comming from the database and recreate the Order struct.
func ReconstituteOrder(id string, status OrderStatus, items []Item, shoppingListID string) *Order {
	return &Order{
		ID:             id,
		Status:         status,
		Items:          items,
		ShoppingListID: shoppingListID,
	}
}
