package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/depot/depotpb"
	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

// toProtoShoppingList will convert the domain ShoppingList struct into a
// protobuf type equivalent needed to by the connectRPC to respond to the caller
func toProtoShoppingList(shoppingList *domain.ShoppingList) *depotpb.ShoppingList {

	// allocate a slice of protobuf Stops the same size as the map of Stops of the
	// shopping list. It'll needed to be added to the pb Shopping List
	pbStops := make([]*depotpb.Stop, 0, len(shoppingList.Stops))

	// range over the map to copy each Stop in the domain Shopping List. We'll need
	// to create protobuf Stops and protobut Items that correspond to the domain
	for storeID, stop := range shoppingList.Stops {

		// allocate a slice of protobuf Items the size size of this current one
		pbItems := make([]*depotpb.Item, 0, len(stop.Items))

		// add the items to the pb slice
		for _, item := range stop.Items {

			// generate a protobuf Item
			pbItem := &depotpb.Item{
				ProductId: item.ProductID,
				Quantity:  int32(item.Quantity),
			}

			// add in the current pb Item
			pbItems = append(pbItems, pbItem)
		}

		// loop completed. All pb Items created
		// -------------------------------------

		// creat a protobuf Stop using data from the domain and attach
		// the pb Items slice
		pbStop := &depotpb.Stop{
			StoreId:       storeID,
			StoreName:     stop.StoreName,
			StoreLocation: stop.StoreLocation,
			Items:         pbItems,
		}

		// add in the current Stop to the slice
		pbStops = append(pbStops, pbStop)
	}

	// loop completed. All pb Stops created and added to the slice

	// construct a protobuf Shopping List
	return &depotpb.ShoppingList{
		Id:      shoppingList.ID,
		OrderId: shoppingList.OrderID,
		Status:  string(shoppingList.Status),
		Stops:   pbStops,
	}
}
