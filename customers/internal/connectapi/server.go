package connectapi

import "github.com/dupakarovsky/eda-storefront/customers/internal/application"

// Server defines a RPC server type that can receive external requests from a
// caller and communicate the request via the application and return responses.
type Server struct {
	registerCustomerUC  *application.RegisterCustomer
	changeSmsNumberUC   *application.ChangeSmsNumber
	enableCustomerUC    *application.EnableCustomer
	disableCustomerUC   *application.DisableCustomer
	authorizeCustomerUC *application.AuthorizeCustomer
	getCustomerUC       *application.GetCustomer
}

// NewServer instantiate a new Server type
func NewServer(
	registerCustomerUC *application.RegisterCustomer,
	changeSmsNumberUC *application.ChangeSmsNumber,
	enableCustomerUC *application.EnableCustomer,
	disableCustomerUC *application.DisableCustomer,
	authorizeCustomerUC *application.AuthorizeCustomer,
	getCustomerUC *application.GetCustomer,
) *Server {
	return &Server{
		registerCustomerUC:  registerCustomerUC,
		changeSmsNumberUC:   changeSmsNumberUC,
		enableCustomerUC:    enableCustomerUC,
		disableCustomerUC:   disableCustomerUC,
		authorizeCustomerUC: authorizeCustomerUC,
		getCustomerUC:       getCustomerUC,
	}
}
