package domain

// Item defines an entry in an order with it's associated
// product ID, store ID and quanitty.Order's Item needs the
// StoreID as it needs to forward it to depot.AddItem per unit.
// depot groups by Items by Stops (storeID) and "ordering" is the
// one that provides it
type Item struct {
	ProductID string
	StoreID   string
	Quanity   int
}

// Order defines an order made by customer in the system when an
// BaskeCheckout happens. ShoppingListID stores the id value returned
// by depot.CreateShoppingList
type Order struct {
	ID             string
	Status         OrderStatus
	Items          []Item
	ShoppingListID string
}
