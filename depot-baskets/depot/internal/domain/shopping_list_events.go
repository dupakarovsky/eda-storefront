// depot/internal/domain/events.go
package domain

import "time"

const (
	ShoppingListCreatedEvent   string = "ShoppingListCreated"
	ItemAddedEvent             string = "ItemAdded"
	ItemRemovedEvent           string = "ItemRemoved"
	ShoppingListInitiatedEvent string = "ShoppingListInitiated"
	ShoppingListCompletedEvent string = "ShoppingListCompleted"
	ShoppingListCanceledEvent  string = "ShoppingListCanceled"
)

type Event interface {
	AggregateID() string
	EventName() string
	OccurredAt() time.Time
}

type eventBase struct {
	aggregateID string
	occurredAt  time.Time
}

func (e eventBase) AggregateID() string {
	return e.aggregateID
}

func (e eventBase) OccurredAt() time.Time {
	return e.occurredAt
}

//============================================================================
// ShoppingList  events
//============================================================================

// ShoppingListCreated implenets the Events and defines when
// a new ShoppingList is created
type ShoppingListCreated struct {
	eventBase
	OrderID string
}

func (ShoppingListCreated) EventName() string {
	return ShoppingListCreatedEvent
}

//-----------------------------------------------------------------------------

// ItemAdded defines when a new new item is added to the Shopping list
type ItemAdded struct {
	eventBase
	StoreID   string
	ProductID string
	Quantity  int
}

func (ItemAdded) EventName() string {
	return ItemAddedEvent
}

//-----------------------------------------------------------------------------

// ItemRemoved defines when a new Item is removed from the Shopping list
type ItemRemoved struct {
	eventBase
	StoreID   string
	ProductID string
}

func (ItemRemoved) EventName() string {
	return ItemRemovedEvent
}

//-----------------------------------------------------------------------------

// ShoppingListInitiated defines when a ...
type ShoppingListInitiated struct {
	eventBase
}

func (ShoppingListInitiated) EventName() string {
	return ShoppingListInitiatedEvent
}

//-----------------------------------------------------------------------------

// ShoppingListCompleted defines the Event of a shopping list changing  it's
// state to completed
type ShoppingListCompleted struct {
	eventBase
}

func (ShoppingListCompleted) EventName() string {
	return ShoppingListCompletedEvent
}

//-----------------------------------------------------------------------------

// ShoppingListCanceled defines the Event of a shopping list changing it's
// state to canceled
type ShoppingListCanceled struct {
	eventBase
}

func (ShoppingListCanceled) EventName() string {
	return ShoppingListCanceledEvent
}
