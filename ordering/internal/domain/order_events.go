package domain

import "time"

const (
	OrderCreatedEvent  string = "OrderCreated"
	OrderReadiedEvent  string = "OrderReadied"
	OrderCanceledEvent string = "OrderCanceled"
)

// Event defines the behavior for any concrete type to implement
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

//-----------------------------------------------------------------------------

//============================================================================
// ORDERING events
//============================================================================

// OrderCreated defines a concrete Event type when a new order is generated
// carring the list of Items from the order
type OrderCreated struct {
	eventBase
	Items []Item
}

func (OrderCreated) EventName() string {
	return OrderCreatedEvent
}

//-----------------------------------------------------------------------------

// OrderReadied defines the concrete Event type when a
// order is set to status "readied"
type OrderReadied struct {
	eventBase
}

func (OrderReadied) EventName() string {
	return OrderReadiedEvent
}

//-----------------------------------------------------------------------------

// OrderCanceled defies a concrete Event type that happens when an
// order is cancelled.
type OrderCanceled struct {
	eventBase
}

func (OrderCanceled) EventName() string {
	return OrderCanceledEvent
}
