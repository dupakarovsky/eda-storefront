package events

import (
	"fmt"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

const serviceName = "stores"

// SubjectFor will construct a NATS subject (<service>.<aggragate>.<event>)
// from a service name ("stores") and an aggragate extracted from the concrete Event type
func SubjectFor(event domain.Event) (string, error) {

	// get the aggregate string from the event type
	aggregate, err := aggregateFor(event)
	if err != nil {
		return "", err
	}

	// return a NATS subject to publish under.
	return foundationnats.Subject(serviceName, aggregate, event.EventName()), nil
}

// aggregateFor will return the aggragate type (store or product) based on the Event
// concrete type it receives.
func aggregateFor(event domain.Event) (string, error) {
	switch event.(type) {

	case domain.StoreCreated, domain.StoreParticipationEnabled, domain.StoreParticipationDisabled, domain.StoreRebranded:
		return "store", nil

	case domain.ProductAdded, domain.ProductRebranded, domain.ProductPriceIncreased, domain.ProductPriceDecreased, domain.ProductRemoved:
		return "product", nil

	default:
		return "", fmt.Errorf("no aggregate mapping define for event type %T", event)
	}

}
