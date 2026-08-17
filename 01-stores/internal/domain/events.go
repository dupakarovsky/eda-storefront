package domain

import "time"

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

// StoreCreated defines a type that implements Events while also carring
// the store's Name and Location
type StoreCreated struct {
	eventBase
	Name     string
	Location string
}

func (StoreCreated) EventName() string {
	return StoreCreatedEvent
}
