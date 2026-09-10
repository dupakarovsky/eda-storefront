// baskets/internal/events/subject.go
package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

const serviceName = "baskets"

// SubjectFor sets the stream subject for the Event to publish to
func SubjectFor(event domain.Event) (string, error) {

	// get the aggregate's name
	aggregate, err := aggregateFor(event)
	if err != nil {
		return "", err
	}

	// construct the stream subject from the "serviceName", aggreate and event name
	return foundationnats.Subject(serviceName, aggregate, event.EventName()), nil
}

// aggregateFor will return the aggreates's name based on the Event type
func aggregateFor(event domain.Event) (string, error) {
	switch event.(type) {

	case domain.BasketStarted, domain.ItemAdded, domain.ItemRemoved, domain.BasketCheckedOut, domain.BasketCanceled:
		return "basket", nil

	default:
		return "", fmt.Errorf("no aggregate mapping defined for event type %T", event)
	}
}
