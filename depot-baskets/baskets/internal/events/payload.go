// baskets/internal/events/payload.go
package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

type BasketStartedPayload struct{}

type ItemAddedPayload struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type ItemRemovedPayload struct {
	ProductID string `json:"product_id"`
}

type BasketCheckedOutPayload struct{}

type BasketCanceledPayload struct{}

// toPayload will conver the Event type into it's proper transport type
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
