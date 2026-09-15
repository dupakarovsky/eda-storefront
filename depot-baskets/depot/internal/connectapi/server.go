// depot/internal/connectapi/server.go
package connectapi

import "github.com/dupakarovsky/eda-storefront/depot/internal/application"

// Server is the ConnectRPC server that will bridge the external request with the
// use case handlers from the application layer
type Server struct {
	createShoppingListUC   *application.CreateShoppingList
	addItemUC              *application.AddItem
	removeItemUC           *application.RemoveItem
	initiateShoppingListUC *application.InitiateShoppingList
	completeShoppingListUC *application.CompleteShoppingList
	cancelShoppingListUC   *application.CancelShoppingList
	getShoppingListUC      *application.GetShoppingList
}

// NewServer instantiates a new Server struct
func NewServer(
	createShoppingListUC *application.CreateShoppingList,
	addItemUC *application.AddItem,
	removeItemUC *application.RemoveItem,
	initiateShoppingListUC *application.InitiateShoppingList,
	completeShoppingListUC *application.CompleteShoppingList,
	cancelShoppingListUC *application.CancelShoppingList,
	getShoppingListUC *application.GetShoppingList,
) *Server {
	return &Server{
		createShoppingListUC:   createShoppingListUC,
		addItemUC:              addItemUC,
		removeItemUC:           removeItemUC,
		initiateShoppingListUC: initiateShoppingListUC,
		completeShoppingListUC: completeShoppingListUC,
		cancelShoppingListUC:   cancelShoppingListUC,
		getShoppingListUC:      getShoppingListUC,
	}
}
