package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/depot/depotpb"
	"github.com/google/uuid"
)

// CreateShoppingList will take a rpc message CreateShoppingListRequest, connect to the grpc server and emit a CreateShoppingListResponse message
func (s *Server) CreateShoppingList(ctx context.Context, req *connect.Request[depotpb.CreateShoppingListRequest]) (*connect.Response[depotpb.CreateShoppingListResponse], error) {

	// generate an unique ID for the shopping list
	uuid := uuid.NewString()

	// call the use case to create a shopping list. creates domain.ShoppingList type
	shoppingList, err := s.createShoppingListUC.Execute(ctx, uuid)
	if err != nil {
		return nil, mapError(err)
	}

	// emit a protobuf response to the caller with the shopping list as message
	return connect.NewResponse(&depotpb.CreateShoppingListResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// CreateShoppingList will take a rpc AddItemRequest message , connect to the grpc server and emit a AddItemResponse message
func (s *Server) AddItem(ctx context.Context, req *connect.Request[depotpb.AddItemRequest]) (*connect.Response[depotpb.AddItemResponse], error) {

	// receve a rpc Request containing the ShoppingListID, the StoreID and the ProductId and
	// call the addItemUC to receive an updated shoppingList
	shoppingList, err := s.addItemUC.Execute(ctx, req.Msg.ShoppingListId, req.Msg.StoreId, req.Msg.ProductId)
	if err != nil {
		return nil, mapError(err)
	}

	// convert the domain shipping list to an rpc type and respond
	return connect.NewResponse(&depotpb.AddItemResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// RemoveItem will take a rpc RemoveItemRequest message , connect to the grpc server and emit a RemoveItemResponse message
func (s *Server) RemoveItem(ctx context.Context, req *connect.Request[depotpb.RemoveItemRequest]) (*connect.Response[depotpb.RemoveItemResponse], error) {

	shoppingList, err := s.removeItemUC.Execute(ctx, req.Msg.ShoppingListId, req.Msg.StoreId, req.Msg.ProductId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&depotpb.RemoveItemResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// InitiateShoppingList will take a rpc InitiateShoppingListRequest message , connect to the grpc server and emit a InitiateShoppingListResponse message
func (s *Server) InitiateShoppingList(ctx context.Context, req *connect.Request[depotpb.InitiateShoppingListRequest]) (*connect.Response[depotpb.InitiateShoppingListResponse], error) {

	shoppingList, err := s.initiateShoppingListUC.Execute(ctx, req.Msg.ShoppingListId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&depotpb.InitiateShoppingListResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// CompleteShoppingList will take a rpc CompleteShoppingListRequest message , connect to the grpc server and emit a CompleteShoppingListResponse message
func (s *Server) CompleteShoppingList(ctx context.Context, req *connect.Request[depotpb.CompleteShoppingListRequest]) (*connect.Response[depotpb.CompleteShoppingListResponse], error) {

	shoppingList, err := s.completeShoppingListUC.Execute(ctx, req.Msg.ShoppingListId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&depotpb.CompleteShoppingListResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// CancelShoppingList will take a rpc CancelShoppingListRequest message , connect to the grpc server and emit a CancelShoppingListResponse message
func (s *Server) CancelShoppingList(ctx context.Context, req *connect.Request[depotpb.CancelShoppingListRequest]) (*connect.Response[depotpb.CancelShoppingListResponse], error) {

	shoppingList, err := s.cancelShoppingListUC.Execute(ctx, req.Msg.ShoppingListId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&depotpb.CancelShoppingListResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}

// GetShoppingList will take a rpc GetShoppingListRequest message , connect to the grpc server and emit a GetShoppingListResponse message
func (s *Server) GetShoppingList(ctx context.Context, req *connect.Request[depotpb.GetShoppingListRequest]) (*connect.Response[depotpb.GetShoppingListResponse], error) {

	shoppingList, err := s.getShoppingListUC.Execute(ctx, req.Msg.ShoppingListId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&depotpb.GetShoppingListResponse{
		ShoppingList: toProtoShoppingList(shoppingList),
	}), nil
}
