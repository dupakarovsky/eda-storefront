package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/stores/storespb"
	"github.com/google/uuid"
)

//----------------------------------------------------------------
// PRODUCT HANDLERS
//----------------------------------------------------------------

// AddProduct will take an rpc Request message, connect to the repository for persistance
// than emit a AddProduct event
func (s *Server) AddProduct(ctx context.Context, req *connect.Request[storespb.AddProductRequest]) (*connect.Response[storespb.AddProductResponse], error) {

	// define a new uuid for the product
	id := uuid.New().String()

	// try to create a product
	product, err := s.addProductUC.Execute(ctx, id, req.Msg.StoreId, req.Msg.Name, req.Msg.Description, req.Msg.Sku, req.Msg.Price)
	if err != nil {
		return nil, mapError(err)
	}

	// product created, persisted and event published
	// ----------------------------------------------

	// respond to client
	response := connect.NewResponse(&storespb.AddProductResponse{
		// convert domain type to protobuf
		Product: toProtoProduct(product),
	})

	return response, nil

}

// RebrandProduct will take an rpc Request message, connect to the repository for persistance
// than emit a RebrandProduct event
func (s *Server) RebrandProduct(ctx context.Context, req *connect.Request[storespb.RebrandProductRequest]) (*connect.Response[storespb.RebrandProductResponse], error) {

	// execute the product rebranding
	product, err := s.rebrandProductUC.Execute(ctx, req.Msg.ProductId, req.Msg.Name, req.Msg.Description)
	if err != nil {
		return nil, mapError(err)
	}

	// emite a response to caller
	return connect.NewResponse(&storespb.RebrandProductResponse{
		Product: toProtoProduct(product),
	}), nil
}

// IncreaseProductPrice will take an rpc Request message, connect to the repository for persistance
// than emit a IncreasePrice event
func (s *Server) IncreaseProductPrice(ctx context.Context, req *connect.Request[storespb.IncreaseProductPriceRequest]) (*connect.Response[storespb.IncreaseProductPriceResponse], error) {

	// execute the price increase
	product, err := s.increaseProductPriceUC.Execute(ctx, req.Msg.ProductId, req.Msg.Price)
	if err != nil {
		return nil, mapError(err)
	}

	// emit the response to the caller
	return connect.NewResponse(&storespb.IncreaseProductPriceResponse{
		Product: toProtoProduct(product),
	}), nil
}

// DecreaseProductPrice will take an rpc Request message, connect to the repository for persistance
// than emit a DecreasePrice event
func (s *Server) DecreaseProductPrice(ctx context.Context, req *connect.Request[storespb.DecreaseProductPriceRequest]) (*connect.Response[storespb.DecreaseProductPriceResponse], error) {

	// execute price decrease
	product, err := s.decreaseProductPriceUC.Execute(ctx, req.Msg.ProductId, req.Msg.Price)
	if err != nil {
		return nil, mapError(err)
	}

	// emite a response to the caller
	return connect.NewResponse(&storespb.DecreaseProductPriceResponse{
		Product: toProtoProduct(product),
	}), nil
}

// RemoveProduct will take an rpc Request message, connect to the repository for persistance
// than emit a Remove event
func (s *Server) RemoveProduct(ctx context.Context, req *connect.Request[storespb.RemoveProductRequest]) (*connect.Response[storespb.RemoveProductResponse], error) {

	// execute product removal
	if err := s.removeProductUC.Execute(ctx, req.Msg.ProductId); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&storespb.RemoveProductResponse{}), nil
}

// ListProducts will connect directly to the repository to retrieve a list of products from a particular store
// as it does not need a
func (s *Server) ListProducts(ctx context.Context, req *connect.Request[storespb.ListProductsRequest]) (*connect.Response[storespb.ListProductsResponse], error) {

	products, err := s.listProductsUC.Execute(ctx, req.Msg.StoreId)
	if err != nil {
		return nil, mapError(err)
	}

	protoProducts := make([]*storespb.Product, 0, len(products))
	for _, product := range products {
		protoProducts = append(protoProducts, toProtoProduct(product))
	}

	return connect.NewResponse(&storespb.ListProductsResponse{
		Products: protoProducts,
	}), nil

}
