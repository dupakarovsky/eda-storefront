package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/stores/internal/application"
	"github.com/dupakarovsky/eda-storefront/stores/storespb"
	"github.com/google/uuid"
)

// Server defines a struct that has the ability to create a Store
// The Server struct needs to implement all methods from the StoresServiceHandler
// interface api.connect in order to make grcp requests
type Server struct {
	createStoreUC  *application.CreateStore
	rebrandStoreUC *application.RebrandStore
}

// NewServer instantiates a new server
func NewServer(createStoreUC *application.CreateStore, rebrandStore *application.RebrandStore) *Server {
	return &Server{
		createStoreUC:  createStoreUC,
		rebrandStoreUC: rebrandStore,
	}
}

// CreateStore will take a rpc message Request, connect to the grpc server and emit a CreateStoreResponse message
func (s *Server) CreateStore(ctx context.Context, req *connect.Request[storespb.CreateStoreRequest]) (*connect.Response[storespb.CreateStoreResponse], error) {

	// create a id for the store
	id := uuid.NewString()

	// run the Execute function to create a Store. It'll connect to the gRPC server before
	// executing the command
	store, err := s.createStoreUC.Execute(ctx, id, req.Msg.Name,
		req.Msg.Location)
	if err != nil {
		// failed to connect to the server
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// create a response message in the CreateStoreResponse format
	response := connect.NewResponse(&storespb.CreateStoreResponse{
		// use a convertion function to convert a domain type to a storepb type
		Store: toProtoStore(store),
	})

	return response, nil
}

//----------------------------------------------------------------
// Implement the stubs statisfy the StoresServiceHandler Interface
//----------------------------------------------------------------

// Rebrand will take a rpc message Request, connect to the grpc server and emit a RebrandStoreResponse message
func (s *Server) RebrandStore(ctx context.Context, req *connect.Request[storespb.RebrandStoreRequest]) (*connect.Response[storespb.RebrandStoreResponse], error) {

	// execute the store rebranding
	store, err := s.rebrandStoreUC.Execute(ctx, req.Msg.StoreId, req.Msg.Name)
	if err != nil {
		// failed to connect to the server
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// emite a response to the caller
	response := connect.NewResponse(&storespb.RebrandStoreResponse{
		// use a convertion function to convert a domain type to a storepb type
		Store: toProtoStore(store),
	})

	return response, nil
}

func (s *Server) EnableStoreParticipation(ctx context.Context, req *connect.Request[storespb.EnableStoreParticipationRequest]) (*connect.Response[storespb.EnableStoreParticipationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) DisableStoreParticipation(ctx context.Context, req *connect.Request[storespb.DisableStoreParticipationRequest]) (*connect.Response[storespb.DisableStoreParticipationResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) AddProduct(ctx context.Context, req *connect.Request[storespb.AddProductRequest]) (*connect.Response[storespb.AddProductResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) RebrandProduct(ctx context.Context, req *connect.Request[storespb.RebrandProductRequest]) (*connect.Response[storespb.RebrandProductResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) IncreaseProductPrice(ctx context.Context, req *connect.Request[storespb.IncreaseProductPriceRequest]) (*connect.Response[storespb.IncreaseProductPriceResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) DecreaseProductPrice(ctx context.Context, req *connect.Request[storespb.DecreaseProductPriceRequest]) (*connect.Response[storespb.DecreaseProductPriceResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) RemoveProduct(ctx context.Context, req *connect.Request[storespb.RemoveProductRequest]) (*connect.Response[storespb.RemoveProductResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) GetStore(ctx context.Context, req *connect.Request[storespb.GetStoreRequest]) (*connect.Response[storespb.GetStoreResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *Server) ListProducts(ctx context.Context, req *connect.Request[storespb.ListProductsRequest]) (*connect.Response[storespb.ListProductsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
