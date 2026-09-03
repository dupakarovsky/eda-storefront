package domain

import "time"

// (corrected constants block)
const (
	StoreCreatedEvent               = "StoreCreated"
	StoreParticipationEnabledEvent  = "StoreParticipationEnabled"
	StoreParticipationDisabledEvent = "StoreParticipationDisabled"
	StoreRebrandedEvent             = "StoreRebranded"

	ProductAddedEvent          = "ProductAdded"
	ProductRebrandedEvent      = "ProductRebranded"
	ProductPriceIncreasedEvent = "ProductPriceIncreased"
	ProductPriceDecreasedEvent = "ProductPriceDecreased"
	ProductRemovedEvent        = "ProductRemoved"
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
// STORE events
//============================================================================

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

// StoreRebranded updates the Store name and emits an StoreRebrandEvent
type StoreRebranded struct {
	eventBase
	Name string
}

func (StoreRebranded) EventName() string {
	return StoreRebrandedEvent
}

//-----------------------------------------------------------------------------

//============================================================================
// PRODUCT events
//============================================================================

// ProductAdded implentes Events while store information about a Product
type ProductAdded struct {
	eventBase
	StoreID     string
	Name        string
	Description string
	SKU         string
	Price       float64
}

func (ProductAdded) EventName() string {
	return ProductAddedEvent
}

//-----------------------------------------------------------------------------

// ProductRebranded implements the Events while storing the new product Name
// and Description
type ProductRebranded struct {
	eventBase
	Name        string
	Description string
}

func (ProductRebranded) EventName() string {
	return ProductRebrandedEvent
}

//-----------------------------------------------------------------------------

// ProductPriceIncreased implements the Events interface while storing the
// price increase difference in a product
type ProductPriceIncreased struct {
	eventBase
	Delta float64
}

func (ProductPriceIncreased) EventName() string {
	return ProductPriceIncreasedEvent
}

//-----------------------------------------------------------------------------

// ProductPriceDecreased implements the Events interface while storing the
// price decrease difference in a product
type ProductPriceDecreased struct {
	eventBase
	Delta float64
}

func (ProductPriceDecreased) EventName() string {
	return ProductPriceDecreasedEvent
}

//-----------------------------------------------------------------------------

// ProductRemoved implement the Event interface
type ProductRemoved struct {
	eventBase
}

func (ProductRemoved) EventName() string {
	return ProductRemovedEvent
}
