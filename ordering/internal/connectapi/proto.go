package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	"github.com/dupakarovsky/eda-storefront/ordering/orderingpb"
)

// toProtoShoppingList will convert the domain Order struct into a
// protobuf type equivalent needed by the connectRPC to respond to the caller

func toProtoOrder(order *domain.Order) *orderingpb.Order {

	// create a slice of ordering protobuf Items of the same size
	pbItems := make([]*orderingpb.Item, 0, len(order.Items))
	// map the order.Item to a orderingpb.Item slice
	for _, item := range order.Items {
		pbItems = append(pbItems, &orderingpb.Item{
			ProductId: item.ProductID,
			StoreId:   item.StoreID,
			Quantity:  int32(item.Quantity),
		})
	}

	// construct an orderingpb.Order type and return it
	return &orderingpb.Order{
		Id:             order.ID,
		Status:         string(order.Status),
		Items:          pbItems,
		ShoppingListId: order.ShoppingListID,
	}

}
