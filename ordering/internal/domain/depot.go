package domain

import "context"

// DepotClient defines the methods any concrete type needs to have to
// be able to communicate with the "depot" service via RPC
type DepotClient interface {
	CreateShoppingList(ctx context.Context, orderID string) (string, error)
	AddItem(ctx context.Context, shoppingListID, storeID, productID string) error
	GetShoppingList(ctx context.Context, shoppingListID string) (map[string]int, error)
	CancelShoppingList(ctx context.Context, shoppingListID string) error
}
