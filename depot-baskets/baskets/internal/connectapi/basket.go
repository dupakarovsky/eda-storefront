package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/baskets/basketspb"
	"github.com/google/uuid"
)

// StartBasket will take a rpc message StartBasketRequest, connect to the grpc server and emit a StartBasketResponse message
func (s *Server) StartBasket(ctx context.Context, req *connect.Request[basketspb.StartBasketRequest]) (*connect.Response[basketspb.StartBasketResponse], error) {

	// generate a new unique ID for the basket to be created
	id := uuid.NewString()

	// call the Execute application handler to create the domain.Basket, save it to the database
	// and publisht he event to the jetstream
	basket, err := s.startBasketUC.Execute(ctx, id)
	if err != nil {
		return nil, mapError(err)
	}

	// handler creates a domain.Basket. Now convert it to
	// a proto buf type to respond to the caller
	// -----------------------------------------------------

	// return the response to the ca
	return connect.NewResponse(&basketspb.StartBasketResponse{
		Basket: toProtoBasket(basket),
	}), nil
}

// AddItem will take a rpc message AddItemRequest, connect to the grpc server and emit a AddItemResponse message
func (s *Server) AddItem(ctx context.Context, req *connect.Request[basketspb.AddItemRequest]) (*connect.Response[basketspb.AddItemResponse], error) {

	basket, err := s.addItemUC.Execute(ctx, req.Msg.BasketId, req.Msg.ProductId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&basketspb.AddItemResponse{
		Basket: toProtoBasket(basket),
	}), nil
}

// RemoveItem will take a rpc message RemoveItemRequest, connect to the grpc server and emit a AddItemResponse message
func (s *Server) RemoveItem(ctx context.Context, req *connect.Request[basketspb.RemoveItemRequest]) (*connect.Response[basketspb.RemoveItemResponse], error) {

	basket, err := s.removeItemUC.Execute(ctx, req.Msg.BasketId, req.Msg.ProductId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&basketspb.RemoveItemResponse{
		Basket: toProtoBasket(basket),
	}), nil
}

// Checkout will take a rpc message CheckoutRequest, connect to the grpc server and emit a CheckoutResponse message
func (s *Server) Checkout(ctx context.Context, req *connect.Request[basketspb.CheckoutRequest]) (*connect.Response[basketspb.CheckoutResponse], error) {

	basket, err := s.checkoutUC.Execute(ctx, req.Msg.BasketId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&basketspb.CheckoutResponse{
		Basket: toProtoBasket(basket),
	}), nil
}

// CancelBasket will take a rpc message CancelBasketRequest, connect to the grpc server and emit a CancelBasketResponse message
func (s *Server) CancelBasket(ctx context.Context, req *connect.Request[basketspb.CancelBasketRequest]) (*connect.Response[basketspb.CancelBasketResponse], error) {

	basket, err := s.cancelBasketUC.Execute(ctx, req.Msg.BasketId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&basketspb.CancelBasketResponse{
		Basket: toProtoBasket(basket),
	}), nil
}

// GetBasket will take a rpc message GetBasketRequest, connect to the grpc server and emit a GetBasketResponse message
func (s *Server) GetBasket(ctx context.Context, req *connect.Request[basketspb.GetBasketRequest]) (*connect.Response[basketspb.GetBasketResponse], error) {

	basket, err := s.getBasketUC.Execute(ctx, req.Msg.BasketId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&basketspb.GetBasketResponse{
		Basket: toProtoBasket(basket),
	}), nil
}
