// customers/internal/application/get_customer_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

func TestGetCustomer_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)

	seeded := registerTestCustomer(t, mod, "get")

	customer, err := mod.GetCustomerUC.Execute(t.Context(), seeded.ID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if customer.ID != seeded.ID || customer.Name != "Alice Smith" || customer.SmsNumber != "+5531999990000" || !customer.Enabled {
		t.Errorf("customer fields not as expected: %+v", customer)
	}
}

func TestGetCustomer_Execute_ReturnsNotFoundForUnknownID(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)

	_, err := mod.GetCustomerUC.Execute(t.Context(), uniqueID("get-unknown"))
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown customer, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}
}
