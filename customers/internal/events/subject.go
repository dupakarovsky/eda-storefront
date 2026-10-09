package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
)

const serviceName = "customers"

// SubjectFor will construct a NATS subject in the format <service>.<aggragate>.<event>
// from a service name ("customers") and an aggragate extracted from the concrete Event type
func SubjectFor(event domain.Event) (string, error) {

	// get the aggregate string from the event type
	aggregate, err := aggregateFor(event)
	if err != nil {
		return "", err
	}

	// return a NATS subject to publish under.
	return foundationnats.Subject(serviceName, aggregate, event.EventName()), nil
}

//-----------------------------------------------------------------------------

// aggregateFor will return the aggragate type based on
// the concrete Event type it receives.
func aggregateFor(event domain.Event) (string, error) {
	switch event.(type) {

	case domain.CustomerRegistered, domain.CustomerSmsChanged, domain.CustomerEnabled, domain.CustomerDisabled:
		return "customer", nil

	default:
		return "", fmt.Errorf("no aggregate mapping define for event type %T", event)
	}

}
