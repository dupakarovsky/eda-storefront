package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/stores/internal/application"
)

// Server defines a struct that has the ability to create a Store
// The Server struct needs to implement all methods from the StoresServiceHandler
// interface api.connect in order to make grcp requests
type Server struct {
	createStoreUC               *application.CreateStore
	getStoreUC                  *application.GetStore
	rebrandStoreUC              *application.RebrandStore
	enableStoreParticipationUC  *application.EnableStoreParticipation
	disableStoreParticipationUC *application.DisableStoreParticipation
	addProductUC                *application.AddProduct
	rebrandProductUC            *application.RebrandProduct
	increaseProductPriceUC      *application.IncreaseProductPrice
	decreaseProductPriceUC      *application.DecreaseProductPrice
	removeProductUC             *application.RemoveProduct
	listProductsUC              *application.ListProducts
}

// NewServer instantiates a new server
func NewServer(
	createStoreUC *application.CreateStore,
	getStoreUC *application.GetStore,
	rebrandStore *application.RebrandStore,
	enableStoreParticipationUC *application.EnableStoreParticipation,
	disableStoreParticipationUC *application.DisableStoreParticipation,
	addProductUC *application.AddProduct,
	rebrandProductUC *application.RebrandProduct,
	increaseProductPriceUC *application.IncreaseProductPrice,
	decreaseProductPriceUC *application.DecreaseProductPrice,
	removeProductUC *application.RemoveProduct,
	listProductsUC *application.ListProducts,
) *Server {
	return &Server{
		createStoreUC:               createStoreUC,
		getStoreUC:                  getStoreUC,
		rebrandStoreUC:              rebrandStore,
		enableStoreParticipationUC:  enableStoreParticipationUC,
		disableStoreParticipationUC: disableStoreParticipationUC,
		addProductUC:                addProductUC,
		rebrandProductUC:            rebrandProductUC,
		increaseProductPriceUC:      increaseProductPriceUC,
		decreaseProductPriceUC:      decreaseProductPriceUC,
		removeProductUC:             removeProductUC,
		listProductsUC:              listProductsUC,
	}
}
