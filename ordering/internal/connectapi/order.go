package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/ordering/orderingpb"
)

// GetOrder will use the call the application.GetOrder that contacts the
// repository and returns an domain.Order. It'll then convert to a proper
// orderingpb.Order type and be replied back to the caller by emiting an
// RPC Response
func (s *Server) GetOrder(ctx context.Context, req *connect.Request[orderingpb.GetOrderRequest]) (*connect.Response[orderingpb.GetOrderResponse], error) {

	// contact the repository and retrieve the order by using the orderID present
	// in the message
	order, err := s.getOrderUC.Execute(ctx, req.Msg.OrderId)
	if err != nil {
		// if it fails, we'll return a wrapped mapError to the caller
		return nil, mapError(err)
	}

	// emit a response
	return connect.NewResponse(&orderingpb.GetOrderResponse{
		Order: toProtoOrder(order),
	}), nil

}

// GetOrder will use the call the application.CancelOrder that contacts the
// repository to: locate the order > cancel it > save it back. Then it'll
// emit an RPC request to the `depto` service requesting the cancelation of the
// shopping list linked to this order. Finally it'll publish the OrderCancel
// event to the stream and return the order
// Once we get a canceled order back, dIt'll  convert it to a proper
// orderingpb.Order type and be replied back to the caller by emiting an
// RPC Response
func (s *Server) CancelOrder(ctx context.Context, req *connect.Request[orderingpb.CancelOrderRequest]) (*connect.Response[orderingpb.CancelOrderResponse], error) {

	// call execute to cancel the order by using the provided order id
	order, err := s.cancelOrderUC.Execute(ctx, req.Msg.OrderId)
	if err != nil {
		return nil, mapError(err)
	}

	// return the canceled order via rpc
	return connect.NewResponse(&orderingpb.CancelOrderResponse{
		Order: toProtoOrder(order),
	}), nil
}
