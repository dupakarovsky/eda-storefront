package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

// ItemPayload defines a type a domain.Item will be converted
// to in order to be transported to the stream.
// It'll be embeded into an OrderCreatedPayload
type ItemPayload struct {
	ProductID string `json:"product_id"`
	StoreID   string `json:"store_id"`
	Quantity  int    `json:"quantity"`
}

// OrderCreatedPayload is the concrete type to be converted to when
// an OrderCreated Event is to be published to the stream
type OrderCreatedPayload struct {
	Items []ItemPayload `json:"items"`
}

// OrderReadiedPayload is the type to be converted to when
// an OrderReady Event is to be published to the stream
type OrderReadiedPayload struct{}

// OrderCanceledPayload is the type to be converted to when
// an OrderCancel Event is to be published to the stream
type OrderCanceledPayload struct{}

// toPayload will take any domain.Event type, and eveluate
// which concrete type it received and then convert it to it's
// proper Payload type
func toPayload(e domain.Event) (any, error) {

	switch event := e.(type) {
	case domain.OrderCreated:
		itemsPayload := make([]ItemPayload, 0, len(event.Items))
		for _, item := range event.Items {
			itemsPayload = append(itemsPayload, ItemPayload{
				ProductID: item.ProductID,
				StoreID:   item.StoreID,
				Quantity:  item.Quantity,
			})
		}
		return OrderCreatedPayload{
			Items: itemsPayload,
		}, nil

	case domain.OrderReadied:
		return OrderReadiedPayload{}, nil

	case domain.OrderCanceled:
		return OrderCanceledPayload{}, nil

	default:
		return nil, fmt.Errorf("no payload mapping defined for event type %T", event)
	}

}
