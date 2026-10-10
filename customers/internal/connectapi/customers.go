package connectapi

import (
	"context"

	"connectrpc.com/connect"
	"github.com/dupakarovsky/eda-storefront/customers/customerspb"
	"github.com/google/uuid"
)

//----------------------------------------------------------------
// CUSTOMERS HANDLERS
//----------------------------------------------------------------

// RegisterCustomer take a protobuf request and extract its contents to generate
// a new Customer. It'll then use the RegisterCustomer Use Case to save the customer
// to the database and publish the Event to the stream
func (s *Server) RegisterCustomer(ctx context.Context, req *connect.Request[customerspb.RegisterCustomerRequest]) (*connect.Response[customerspb.RegisterCustomerResponse], error) {

	// create a uuid for the Customer
	customerID := uuid.NewString()

	// execute the registerCustomerUC passing in the id, name and sms number to register
	// a new customer
	customer, err := s.registerCustomerUC.Execute(ctx, customerID, req.Msg.Name, req.Msg.SmsNumber)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the client by converting the domain type
	// to a protobuf registred type
	response := connect.NewResponse(&customerspb.RegisterCustomerResponse{
		Customer: toProtoCustomer(customer),
	})

	return response, nil
}

// ChangeSmsNumber take a protobuf request and extract its contents
// locatet the customers id. It'll then use the ChangeSmsNumber to update the customer
// sms number to the databasee and publish the Event to the stream
func (s *Server) ChangeSmsNumber(ctx context.Context, req *connect.Request[customerspb.ChangeSmsNumberRequest]) (*connect.Response[customerspb.ChangeSmsNumberResponse], error) {

	// get the customer ID from the message
	customerID := req.Msg.CustomerId
	smsNumber := req.Msg.SmsNumber

	// call the ChangeSmsNumber Use case to update the number in the database
	// and publish the event
	customer, err := s.changeSmsNumberUC.Execute(ctx, customerID, smsNumber)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the client by converting the domain type
	// to a protobuf registred type
	response := connect.NewResponse(&customerspb.ChangeSmsNumberResponse{
		Customer: toProtoCustomer(customer),
	})

	return response, nil
}

// EnableCustomer take a protobuf request and extract its contents to
// locate the customer id. It'll then use the EnableCustomer Use Case to update the customer
// status to the databasee and publish the Event to the stream
func (s *Server) EnableCustomer(ctx context.Context, req *connect.Request[customerspb.EnableCustomerRequest]) (*connect.Response[customerspb.EnableCustomerResponse], error) {

	// get the customer ID from the message
	customerID := req.Msg.CustomerId

	// call the EnableCustomer Use Case to update the customer status  in the database
	// and publish the event
	customer, err := s.enableCustomerUC.Execute(ctx, customerID)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the client by converting the domain type
	// to a protobuf registred type
	response := connect.NewResponse(&customerspb.EnableCustomerResponse{
		Customer: toProtoCustomer(customer),
	})

	return response, nil
}

// DisableCustomer take a protobuf request and extract its contents to retrieve
// the customer id. It'll then use the DisableCustomer Use Case to update the customer
// status to the databasee and publish the Event to the stream
func (s *Server) DisableCustomer(ctx context.Context, req *connect.Request[customerspb.DisableCustomerRequest]) (*connect.Response[customerspb.DisableCustomerResponse], error) {

	// get the customer ID from the message
	customerID := req.Msg.CustomerId

	// call the DisableCustomer Use Case to update the customer status  in the database
	// and publish the event
	customer, err := s.disableCustomerUC.Execute(ctx, customerID)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the client by converting the domain type
	// to a protobuf registred type
	response := connect.NewResponse(&customerspb.DisableCustomerResponse{
		Customer: toProtoCustomer(customer),
	})

	return response, nil
}

// DisableCustomer take a protobuf request and extract its contents to generate
// a new Customer. It'll then use the DisableCustomer Use Case to update the customer
// status to the databasee and publish the Event to the stream
func (s *Server) AuthorizeCustomer(ctx context.Context, req *connect.Request[customerspb.AuthorizeCustomerRequest]) (*connect.Response[customerspb.AuthorizeCustomerResponse], error) {

	// get the customer ID from the message
	customerID := req.Msg.CustomerId

	// call AuthorizeCustomer usecase to authorize the customer
	err := s.authorizeCustomerUC.Execute(ctx, customerID)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the client. Here the response does not
	// have any data
	response := connect.NewResponse(&customerspb.AuthorizeCustomerResponse{})

	return response, nil
}

// GetCustomer take a protobuf request and extract its contents to locate the
// customer's ID. It'll then use the GetCustomer Use Case to locate the customer
// in the database database and return it.
func (s *Server) GetCustomer(ctx context.Context, req *connect.Request[customerspb.GetCustomerRequest]) (*connect.Response[customerspb.GetCustomerResponse], error) {

	// get the customer ID from the message
	customerID := req.Msg.CustomerId

	// call the GetStore use case
	customer, err := s.getCustomerUC.Execute(ctx, customerID)
	if err != nil {
		return nil, mapError(err)
	}

	// generate a response to the caller
	response := connect.NewResponse(&customerspb.GetCustomerResponse{
		Customer: toProtoCustomer(customer),
	})

	return response, nil
}
