// depot/internal/domain/shopping_list_status.go
package domain

type ShoppingListStatus string

const (
	ShoppingListPendingStatus   ShoppingListStatus = "pending"
	ShoppingListActiveStatus    ShoppingListStatus = "active"
	ShoppingListCompletedStatus ShoppingListStatus = "completed"
	ShoppingListCanceledStatus  ShoppingListStatus = "canceled"
)
