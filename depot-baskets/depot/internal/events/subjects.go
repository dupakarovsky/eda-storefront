// depot/internal/events/subject.go
package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

const serviceName = "depot"

// SubjectFor sets the stream subject for the Event to publish to
func SubjectFor(event domain.Event) (string, error) {

	// get the name of the "aggreate" from the Event
	aggregate, err := aggregateFor(event)
	if err != nil {
		return "", err
	}

	// build a NAT's jetsream subject ("depot.shopping_list.<event>")
	return foundationnats.Subject(serviceName, aggregate, event.EventName()), nil
}

// aggregateFor will return the aggreates's name based on the Event type
func aggregateFor(event domain.Event) (string, error) {
	switch event.(type) {

	// for any event from ShoppingList we return the "shopping_list" aggregate
	case domain.ShoppingListCreated, domain.ItemAdded, domain.ItemRemoved,
		domain.ShoppingListInitiated, domain.ShoppingListCompleted, domain.ShoppingListCanceled:
		return "shopping_list", nil

	default:
		return "", fmt.Errorf("no aggregate mapping defined for event type %T", event)
	}
}
