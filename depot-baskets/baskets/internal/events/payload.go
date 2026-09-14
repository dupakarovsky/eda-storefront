// baskets/internal/events/payload.go
package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

// BasketStartedPayload is the concrete type to be converted to when
// an Event BasketStarted is to be published to the stream
type BasketStartedPayload struct{}

// ItemAddedPayload is the concrete type to be converted to when
// an Event ItemAdded is to be published to the stream
type ItemAddedPayload struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// ItemRemovedPayload is the concrete type to be converted to when
// an Event ItemRemoved is to be published to the stream
type ItemRemovedPayload struct {
	ProductID string `json:"product_id"`
}

// BasketCheckedOutPayload is the concrete type to be converted to when
// an Event BasketCheckedOut is to be published to the stream
type BasketCheckedOutPayload struct{}

// BasketCanceledPayload is the concrete type to be converted to when
// an Event BasketCanceled is to be published to the stream
type BasketCanceledPayload struct{}

// toPayload will convert the domain Event type into it's proper transport type
func toPayload(event domain.Event) (any, error) {

	switch e := event.(type) {

	case domain.BasketStarted:
		return BasketStartedPayload{}, nil

	case domain.ItemAdded:
		return ItemAddedPayload{
			ProductID: e.ProductID,
			Quantity:  e.Quantity,
		}, nil

	case domain.ItemRemoved:
		return ItemRemovedPayload{
			ProductID: e.ProductID,
		}, nil

	case domain.BasketCheckedOut:
		return BasketCheckedOutPayload{}, nil

	case domain.BasketCanceled:
		return BasketCanceledPayload{}, nil

	default:
		return nil, fmt.Errorf("no payload mapping defined for event type %T", event)
	}
}
