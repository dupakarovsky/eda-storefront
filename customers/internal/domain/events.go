package domain

import "time"

const (
	CustomerRegisteredEvent = "CustomerRegistered"
	CustomerSmsChangedEvent = "CustomerSmsChanged"
	CustomerEnabledEvent    = "CustomerEnabled"
	CustomerDisabledEvent   = "CustomerDisabled"
)

// Event defines the behavior needed by any concrete type to be
// categorized as an Event
type Event interface {
	AggregateID() string
	EventName() string
	OccurredAt() time.Time
}

// eventBase defines a type the fullfill two of the methods defined
// by the Event Inteface. All concrete types defined here will
// embed this type
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

// CustomerRegistered defines an Event type that is emmited when
// a customer registers in the system
type CustomerRegistered struct {
	eventBase
	Name      string
	SmsNumber string
}

func (CustomerRegistered) EventName() string {
	return CustomerRegisteredEvent
}

//-------------------------------------------------------------------------------

// CustomerSmsChanged defines an Event type that is emmited when
// a customer updates they're Sms number
type CustomerSmsChanged struct {
	eventBase
	SmsNumber string
}

func (CustomerSmsChanged) EventName() string {
	return CustomerSmsChangedEvent
}

// CustomerSmsChanged defines an Event type that is emmited when
// a customer is enbaled
// -------------------------------------------------------------------------------
type CustomerEnabled struct {
	eventBase
}

func (CustomerEnabled) EventName() string {
	return CustomerEnabledEvent
}

// CustomerSmsChanged defines an Event type that is emmited when
// a customer disabled
// -------------------------------------------------------------------------------
type CustomerDisabled struct {
	eventBase
}

func (CustomerDisabled) EventName() string {
	return CustomerDisabledEvent
}
