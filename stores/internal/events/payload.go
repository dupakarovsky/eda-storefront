package events

import (
	"fmt"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// StoreCreatedPayload will be responsibile for carrying the
// StoreCreated Event between domain boundaries
type StoreCreatedPayload struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

// StoreParticipationEnabledPayload will be responsibile for carrying the
// StoreParticipationEnabled Event between domains boundaries
type StoreParticipationEnabledPayload struct {
	Participating bool `json:"participating"`
}

// StoreParticipationDisabledPayload will be responsibile for carrying the
// StoreParticipationDisabled Event between domains boundaries
type StoreParticipationDisabledPayload struct {
	Participating bool `json:"participating"`
}

type StoreRebrandedPayload struct {
	Name string `json:"name"`
}

// ProductAddedPayload will be responsibile for carrying the
// ProductAdded Event between domains boundaries
type ProductAddedPayload struct {
	StoreID     string  `json:"store_id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	SKU         string  `json:"sku"`
	Price       float64 `json:"price"`
}

// ProductRebrandedPayload will be responsibile for carrying the
// ProductRebranded Event between domains boundaries
type ProductRebrandedPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ProductPriceIncreasedPayload will be esponsibile for carrying the
// ProductPriceIncreased Event between domains boundaries
type ProductPriceIncreasedPayload struct {
	Delta float64 `json:"delta"`
	Price float64 `json:"price"` // new
}

// ProductPriceDecreasedPayload will be responsibile for carrying the
// ProductPriceDecreased Event between domains boundaries
type ProductPriceDecreasedPayload struct {
	Delta float64 `json:"delta"`
	Price float64 `json:"price"` // new
}

// ProductRemovedPayload will be responsibile for carrying the
// ProductPriceDecreased Event between domains boundaries
type ProductRemovedPayload struct{}

// to toPayload converts the Event concrete type to one of the
// Payload types
func toPayload(event domain.Event) (any, error) {

	switch e := event.(type) {

	case domain.StoreCreated:
		return StoreCreatedPayload{
			Name:     e.Name,
			Location: e.Location,
		}, nil

	case domain.StoreParticipationEnabled:
		return StoreParticipationEnabledPayload{
			Participating: e.Participating,
		}, nil

	case domain.StoreParticipationDisabled:
		return StoreParticipationDisabledPayload{
			Participating: e.Participating,
		}, nil

	case domain.StoreRebranded:
		return StoreRebrandedPayload{
			Name: e.Name,
		}, nil

	case domain.ProductAdded:
		return ProductAddedPayload{
			StoreID:     e.StoreID,
			Name:        e.Name,
			Description: e.Description,
			SKU:         e.SKU,
			Price:       e.Price,
		}, nil

	case domain.ProductRebranded:
		return ProductRebrandedPayload{
			Name:        e.Name,
			Description: e.Description,
		}, nil

	case domain.ProductPriceIncreased:
		return ProductPriceIncreasedPayload{
			Delta: e.Delta,
			Price: e.Price, // new
		}, nil

	case domain.ProductPriceDecreased:
		return ProductPriceDecreasedPayload{
			Delta: e.Delta,
			Price: e.Price, // new
		}, nil

	case domain.ProductRemoved:
		return ProductRemovedPayload{}, nil

	default:
		return nil, fmt.Errorf("no payload mapping defined for event type %T", event)
	}
}
