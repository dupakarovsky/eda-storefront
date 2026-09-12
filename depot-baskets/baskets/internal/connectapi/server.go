package connectapi

import "github.com/dupakarovsky/eda-storefront/baskets/internal/application"

// Server is the ConnectRPC server that will bridge the external request with the
// use case handlers from the application layer
type Server struct {
	startBasketUC  *application.StartBasket
	addItemUC      *application.AddItem
	removeItemUC   *application.RemoveItem
	checkoutUC     *application.Checkout
	cancelBasketUC *application.CancelBasket
	getBasketUC    *application.GetBasket
}

// NewServer instantiates a new Server from the application Use Cases
// it receives
func NewServer(
	startBasketUC *application.StartBasket,
	addItemUC *application.AddItem,
	removeItemUC *application.RemoveItem,
	checkoutUC *application.Checkout,
	cancelBasketUC *application.CancelBasket,
	getBasketUC *application.GetBasket,
) *Server {
	return &Server{
		startBasketUC:  startBasketUC,
		addItemUC:      addItemUC,
		removeItemUC:   removeItemUC,
		checkoutUC:     checkoutUC,
		cancelBasketUC: cancelBasketUC,
		getBasketUC:    getBasketUC,
	}
}
