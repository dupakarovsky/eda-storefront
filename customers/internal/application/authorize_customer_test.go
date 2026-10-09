// customers/internal/application/authorize_customer_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

func TestAuthorizeCustomer_Execute_AuthorizesEnabledCustomerAndPublishesNothing(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "authorize")
	before := streamMessageCount(t, ctx, mod)

	if err := mod.AuthorizeCustomerUC.Execute(ctx, seeded.ID); err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if after := streamMessageCount(t, ctx, mod); after != before {
		t.Errorf("expected authorization to publish nothing, got %d new messages", after-before)
	}
}

func TestAuthorizeCustomer_Execute_RejectsDisabledCustomer(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "authorize-disabled")
	if _, err := mod.DisableCustomerUC.Execute(ctx, seeded.ID); err != nil {
		t.Fatalf("failed to disable as test setup: %v", err)
	}

	err := mod.AuthorizeCustomerUC.Execute(ctx, seeded.ID)
	if err == nil {
		t.Fatal("expected Execute to fail for a disabled customer, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerNotAuthorized) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotAuthorized, err)
	}
}

func TestAuthorizeCustomer_Execute_RejectsUnknownCustomer(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)

	err := mod.AuthorizeCustomerUC.Execute(t.Context(), uniqueID("authorize-unknown"))
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown customer, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}
}
