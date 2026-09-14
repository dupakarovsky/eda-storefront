// depot/internal/events/payload.go
package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

// ShoppingListCreatedPayload is the concrete type to be converted to when
// an Event ShoppingListCreated is to be published to the stream
type ShoppingListCreatedPayload struct{}

// ItemAddedPayload is the concrete type to be converted to when
// an Event ItemAdded is to be published to the stream
type ItemAddedPayload struct {
	StoreID   string `json:"store_id"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// ItemRemovedPayload is the concrete type to be converted to when
// an Event ItemRemoved is to be published to the stream
type ItemRemovedPayload struct {
	StoreID   string `json:"store_id"`
	ProductID string `json:"product_id"`
}

// ShoppingListInitiatedPayload is the concrete type to be converted to when
// an Event ShoppingListInitiated is to be published to the stream
type ShoppingListInitiatedPayload struct{}

// ShoppingListCompletedPayload is the concrete type to be converted to when
// an Event ShoppingListCompleted is to be published to the stream
type ShoppingListCompletedPayload struct{}

// ShoppingListCanceledPayload is the concrete type to be converted to when
// an Event ShoppingListCanceled is to be published to the stream
type ShoppingListCanceledPayload struct{}

// toPayload will convert the domain Event type into it's proper transport type
func toPayload(event domain.Event) (any, error) {

	switch e := event.(type) {

	case domain.ShoppingListCreated:
		return ShoppingListCreatedPayload{}, nil

	case domain.ItemAdded:
		return ItemAddedPayload{
			StoreID:   e.StoreID,
			ProductID: e.ProductID,
			Quantity:  e.Quantity,
		}, nil

	case domain.ItemRemoved:
		return ItemRemovedPayload{
			StoreID:   e.StoreID,
			ProductID: e.ProductID,
		}, nil

	case domain.ShoppingListInitiated:
		return ShoppingListInitiatedPayload{}, nil

	case domain.ShoppingListCompleted:
		return ShoppingListCompletedPayload{}, nil

	case domain.ShoppingListCanceled:
		return ShoppingListCanceledPayload{}, nil

	default:
		return nil, fmt.Errorf("no payload mapping defined for event type %T", event)
	}
}
