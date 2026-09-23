package events

import (
	"fmt"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

const serviceName = "ordering"

// SubjectFor sets the stream subject for the Event to publish to
func SubjectFor(event domain.Event) (string, error) {

	// the the aggreate name from the event
	aggregate, err := aggregateFor(event)
	if err != nil {
		return "", err
	}

	// return the subject this event will be published to in the stream
	return foundationnats.Subject(serviceName, aggregate, event.EventName()), nil
}

// aggregateFor will take an event, extract and return the aggreate name
func aggregateFor(event domain.Event) (string, error) {
	switch event.(type) {

	case domain.OrderCreated, domain.OrderReadied, domain.OrderCanceled:
		return "order", nil

	default:
		return "", fmt.Errorf("no aggregate mapping defined for event type %T", event)
	}
}
