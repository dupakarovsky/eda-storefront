package domain

import "time"

const (
	BasketStartedEvent    string = "BasketStarted"
	ItemAddedEvent        string = "ItemAdded"
	ItemRemovedEvent      string = "ItemRemoved"
	BasketCheckedOutEvent string = "BasketCheckedOut"
	BasketCanceledEvent   string = "BasketCanceled"
)

// Event defines the behavior for any concrete type to implement
type Event interface {
	AggregateID() string
	EventName() string
	OccurredAt() time.Time
}

// eventBase is a struct that implements two of the methods needed by
// the Event interface, allowing a concrete type to implement only
// the AggregateID method, when embeding this type
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

//-----------------------------------------------------------------------------

//============================================================================
// Basket events
//============================================================================

// BasketStarted is a concrete type that implement the Events interface
// allowing it to be treated as an Event
type BasketStarted struct {
	eventBase
	Status BasketStatus
}

// EventName returns the this event's needed
func (BasketStarted) EventName() string {
	return BasketStartedEvent
}

//----------------------------------------------------------------

// ItemAdded is a concrete type that implement the Events interface
// allowing it to be treated as an Event
type ItemAdded struct {
	eventBase
	ProductID string
	Quantity  int
}

// EventName returns the this event's needed
func (ItemAdded) EventName() string {
	return ItemAddedEvent
}

//----------------------------------------------------------------

// ItemRemoved is a concrete type that implement the Events interface
// allowing it to be treated as an Event
type ItemRemoved struct {
	eventBase
	ProductID string
}

func (ItemRemoved) EventName() string {
	return ItemRemovedEvent
}

// ----------------------------------------------------------------

// BasketCanceled is a concrete type that implement the Events interface
// allowing it to be treated as an Event
type BasketCanceled struct {
	eventBase
}

func (BasketCanceled) EventName() string {
	return BasketCanceledEvent
}

// ----------------------------------------------------------------

// ItemCheckedOut is an Item residing in the BasketCheckedOut Event
// type.
type ItemCheckedOut struct {
	ProductID string
	StoreID   string
	Quantity  int
}

// ----------------------------------------------------------------

// BasketCheckedOut is a concrete type that implement the Events interface
// allowing it to be treated as an Event
type BasketCheckedOut struct {
	eventBase
	Items []ItemCheckedOut
}

func (BasketCheckedOut) EventName() string {
	return BasketCheckedOutEvent
}
