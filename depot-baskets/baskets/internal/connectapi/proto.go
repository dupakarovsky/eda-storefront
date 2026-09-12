package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/baskets/basketspb"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

// toProtoBasket converts a Basket domain type to a basketpb Basket type
func toProtoBasket(basket *domain.Basket) *basketspb.Basket {

	// make a slice of basketpb the same capacity of the domain Items slice
	items := make([]*basketspb.Item, 0, len(basket.Items))

	// create a slice of basketpb Items from the domain Basket Items
	// slice
	for _, domainItem := range basket.Items {
		pbItem := basketspb.Item{
			ProductId: domainItem.ProductID,
			Quantity:  int32(domainItem.Quantity),
		}
		items = append(items, &pbItem)
	}

	return &basketspb.Basket{
		Id:     basket.ID,
		Status: string(basket.Status),
		Items:  items,
	}
}
