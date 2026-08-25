package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/stores/storespb"
	"github.com/google/uuid"
)

//----------------------------------------------------------------
// STORE HANDLERS
//----------------------------------------------------------------

// CreateStore will take a rpc message Request, connect to the grpc server and emit a CreateStoreResponse message
func (s *Server) CreateStore(ctx context.Context, req *connect.Request[storespb.CreateStoreRequest]) (*connect.Response[storespb.CreateStoreResponse], error) {

	// create a id for the store
	id := uuid.NewString()

	// run the Execute function to create a Store. It'll connect to the gRPC server before
	// executing the command.
	store, err := s.createStoreUC.Execute(ctx, id, req.Msg.Name,
		req.Msg.Location)
	if err != nil {
		// failed to connect to the server. NATS or Postgres are down
		return nil, mapError(err)
	}

	// create a response message in the CreateStoreResponse format
	response := connect.NewResponse(&storespb.CreateStoreResponse{
		// use a convertion function to convert a domain type to a storepb type
		Store: toProtoStore(store),
	})

	return response, nil
}

// Rebrand will take a rpc message Request, connect to the grpc server and emit a RebrandStoreResponse message
func (s *Server) RebrandStore(ctx context.Context, req *connect.Request[storespb.RebrandStoreRequest]) (*connect.Response[storespb.RebrandStoreResponse], error) {

	// execute the store rebranding
	store, err := s.rebrandStoreUC.Execute(ctx, req.Msg.StoreId, req.Msg.Name)
	if err != nil {
		// failed to connect to the server
		return nil, mapError(err)
	}

	// emite a response to the caller
	response := connect.NewResponse(&storespb.RebrandStoreResponse{
		// use a convertion function to convert a domain type to a storepb type
		Store: toProtoStore(store),
	})

	return response, nil
}

// EnableStoreParticipation take an rpc message Request, connect to the serer and emit a EnableStoreParticipation event
func (s *Server) EnableStoreParticipation(ctx context.Context, req *connect.Request[storespb.EnableStoreParticipationRequest]) (*connect.Response[storespb.EnableStoreParticipationResponse], error) {

	// call the EnableStoreParticipation usecase
	store, err := s.enableStoreParticipationUC.Execute(ctx, req.Msg.StoreId)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the caller
	response := connect.NewResponse(&storespb.EnableStoreParticipationResponse{
		Store: toProtoStore(store),
	})

	return response, nil
}

// DisableStoreParticipation takes an rpc message Request, connect to the serer and emit a DisableStoreParticipation event
func (s *Server) DisableStoreParticipation(ctx context.Context, req *connect.Request[storespb.DisableStoreParticipationRequest]) (*connect.Response[storespb.DisableStoreParticipationResponse], error) {

	// call the DisableStoreParticipation usecase
	store, err := s.disableStoreParticipationUC.Execute(ctx, req.Msg.StoreId)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the caller
	response := connect.NewResponse(&storespb.DisableStoreParticipationResponse{
		Store: toProtoStore(store),
	})

	// emit response
	return response, nil
}

func (s *Server) GetStore(ctx context.Context, req *connect.Request[storespb.GetStoreRequest]) (*connect.Response[storespb.GetStoreResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
