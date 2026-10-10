package connectapi

import (
	"github.com/dupakarovsky/eda-storefront/customers/customerspb"
	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

// toProtoCustomer converts a domain.Customer type to an rpc customersbp
// type so data can be send to the client
func toProtoCustomer(customer *domain.Customer) *customerspb.Customer {
	return &customerspb.Customer{
		Id:        customer.ID,
		Name:      customer.Name,
		SmsNumber: customer.SmsNumber,
		Enabled:   customer.Enabled,
	}
}
