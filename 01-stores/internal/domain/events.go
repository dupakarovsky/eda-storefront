package domain

import "time"

const (
	StoreCreatedEvent               = "stores.StoreCretated"
	StoreParticipationEnabledEvent  = "stores.StoreParticipationEnabled"
	StoreParticipationDisabledEvent = "stores.StoreParticipationDisabled"
	StoreRebrandEvent               = "stores.StoreRebrandEvent"
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

//-----------------------------------------------------------------------------

// StoreParticipationEnabled defines a type that implements Events
// and returns a StoreParticipationEnabledEvent as the EventName
type StoreParticipationEnabled struct {
	eventBase
	Participating bool
}

func (e StoreParticipationEnabled) EventName() string {
	return StoreParticipationEnabledEvent
}

//-----------------------------------------------------------------------------

// StoreParticipationDisabled defines a type that implements Events
// and returns a StoreParticipationDisenabledEvent as the EventName
type StoreParticipationDisabled struct {
	eventBase
	Participating bool
}

func (e StoreParticipationDisabled) EventName() string {
	return StoreParticipationDisabledEvent
}

//-----------------------------------------------------------------------------

// StoreRebrand updates the Store name and emits an StoreRebrandEvent
type StoreRebrand struct {
	eventBase
	Name string
}

func (StoreRebrand) EventName() string {
	return StoreRebrandEvent
}
