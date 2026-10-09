// customers/internal/application/disable_customer_test.go
package application_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
)

func TestDisableCustomer_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "disable")
	before := streamMessageCount(t, ctx, mod)

	customer, err := mod.DisableCustomerUC.Execute(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}
	if customer.Enabled {
		t.Error("expected returned customer to be disabled")
	}

	found, err := mod.CustomerRepo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}
	if found.Enabled {
		t.Error("expected persisted customer to be disabled")
	}

	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Fatalf("expected exactly 1 new message, got %d", after-before)
	}

	env := lastEnvelope(t, ctx, mod, "customers.customer.CustomerDisabled")
	if env.AggregateID != seeded.ID {
		t.Errorf("expected last event for %s, got %+v", seeded.ID, env)
	}
}

func TestDisableCustomer_Execute_RejectsAlreadyDisabled(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	seeded := registerTestCustomer(t, mod, "disable-twice")
	if _, err := mod.DisableCustomerUC.Execute(ctx, seeded.ID); err != nil {
		t.Fatalf("failed on first disable: %v", err)
	}

	before := streamMessageCount(t, ctx, mod)

	_, err := mod.DisableCustomerUC.Execute(ctx, seeded.ID)
	if err == nil {
		t.Fatal("expected second Execute to fail, got nil error")
	}
	if !errors.Is(err, domain.ErrCustomerAlreadyDisabled) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerAlreadyDisabled, err)
	}

	if after := streamMessageCount(t, ctx, mod); after != before {
		t.Errorf("expected no new message, got %d", after-before)
	}
}

func TestDisableCustomer_Execute_SerializesConcurrentDisables(t *testing.T) {

	// new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// seed a customer in the database
	seeded := registerTestCustomer(t, mod, "disable-concurrent")
	// get the current number of messages
	before := streamMessageCount(t, ctx, mod)

	// number of concurrent requests
	const callers = 4

	// capture errors slice
	errs := make([]error, callers)

	// sync channel
	start := make(chan struct{})
	var wg sync.WaitGroup

	// genearate 4 goroutines to call DisableCustomerUC
	wg.Add(callers)
	for i := range callers {
		go func() {
			defer wg.Done()
			// block all gourtines until loop is over
			<-start
			// when unlocked; Disable the customer; only 1 should succed
			// the other 3 should fail.
			_, errs[i] = mod.DisableCustomerUC.Execute(ctx, seeded.ID)
		}()
	}

	// let all gorouitnes proceed; first to run acquires the lock
	close(start)

	// wait for all goroutines to finish their turn
	wg.Wait()

	// store the number of successes and rejections
	succeeded, rejected := 0, 0

	// record the number of success and rejections
	for _, err := range errs {
		switch {
		case nil == err:
			succeeded++
		case errors.Is(err, domain.ErrCustomerAlreadyDisabled):
			rejected++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}

	// should have only 1 success; other should fail
	if succeeded != 1 || rejected != callers-1 {
		t.Errorf("expected exactly 1 success and %d rejections, got %d and %d", callers-1, succeeded, rejected)
	}

	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Errorf("expected exactly 1 published event across all callers, got %d", after-before)
	}
}
