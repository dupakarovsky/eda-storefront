// customers/internal/application/enable_customer_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

func TestEnableCustomer_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "enable")
	if _, err := mod.DisableCustomerUC.Execute(ctx, seeded.ID); err != nil {
		t.Fatalf("failed to disable as test setup: %v", err)
	}

	before := streamMessageCount(t, ctx, mod)

	customer, err := mod.EnableCustomerUC.Execute(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}
	if !customer.Enabled {
		t.Error("expected returned customer to be enabled")
	}

	found, err := mod.CustomerRepo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}
	if !found.Enabled {
		t.Error("expected persisted customer to be enabled")
	}

	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Fatalf("expected exactly 1 new message, got %d", after-before)
	}

	env := lastEnvelope(t, ctx, mod, "customers.customer.CustomerEnabled")
	if env.AggregateID != seeded.ID {
		t.Errorf("expected last event for %s, got %+v", seeded.ID, env)
	}
}

func TestEnableCustomer_Execute_RejectsAlreadyEnabled(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "enable-twice")
	before := streamMessageCount(t, ctx, mod)

	_, err := mod.EnableCustomerUC.Execute(ctx, seeded.ID)
	if err == nil {
		t.Fatal("expected Execute to fail on an already enabled customer, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerAlreadyEnabled) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerAlreadyEnabled, err)
	}

	if after := streamMessageCount(t, ctx, mod); after != before {
		t.Errorf("expected no new message, got %d", after-before)
	}
}

func TestEnableCustomer_Execute_RejectsUnknownCustomer(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)

	_, err := mod.EnableCustomerUC.Execute(t.Context(), uniqueID("enable-unknown"))
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown customer, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}
}
