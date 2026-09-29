package connectapi

import "github.com/dupakarovsky/eda-storefront/ordering/internal/application"

// Server defines a RPC server type that can receive external requests from a
// caller and communicate the request via the application and return responses.
// For the Order service, only two Use Cases are needet to take outside
// requests. The the oders are event driven
type Server struct {
	getOrderUC    *application.GetOrder
	cancelOrderUC *application.CancelOrder
}

// NewServer instantiate a new Server type
func NewServer(
	getOrderUC *application.GetOrder,
	cancelOrderUC *application.CancelOrder,
) *Server {
	return &Server{
		getOrderUC:    getOrderUC,
		cancelOrderUC: cancelOrderUC,
	}
}
