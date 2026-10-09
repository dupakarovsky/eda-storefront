package application_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	customersevents "github.com/dupakarovsky/eda-storefront/customers/internal/events"
)

func TestRegisterCustomer_Execute_Succeeds(t *testing.T) {
	// initiliaze a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// regiester an ID for a new customer
	id := uniqueID("register")

	// get the number of messages in the stream
	before := streamMessageCount(t, ctx, mod)

	// create a new customer and save to the database;
	customer, err := mod.RegisterCustomerUC.Execute(ctx, id, "Alice Smith", "+5531999990000")
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify the customer's properties
	if customer.ID != id || customer.Name != "Alice Smith" || customer.SmsNumber != "+5531999990000" || !customer.Enabled {
		t.Errorf("customer fields not set as expected: %+v", customer)
	}

	// query the database to see if it was saved
	found, err := mod.CustomerRepo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("expected the customer to be persisted, got error: %v", err)
	}
	// verify the Name proprty of the returned
	if found.Name != "Alice Smith" {
		t.Errorf("expected persisted name %q, got %q", "Alice Smith", found.Name)
	}

	// now check the number of messages in the stream; expected to have increased by 1
	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Fatalf("expected exactly 1 new message, got %d", after-before)
	}

	// retrieve the last message from the stream; should be a CustomerRegistered Event type
	env := lastEnvelope(t, ctx, mod, "customers.customer.CustomerRegistered")
	if env.AggregateID != id || env.EventName != "CustomerRegistered" {
		t.Errorf("unexpected last event: %+v", env)
	}

	// unmarshal the message into a CustomerRegisteredPayload struct
	var payload customersevents.CustomerRegisteredPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	// Verify the name and sms number
	if payload.Name != "Alice Smith" || payload.SmsNumber != "+5531999990000" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

func TestRegisterCustomer_Execute_RejectsBlankName(t *testing.T) {

	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define an ID for a customer
	id := uniqueID("register-blank")

	// check the number of messages in the stream
	before := streamMessageCount(t, ctx, mod)

	// call the method to register a new customer with a blank name; expected to fail
	customer, err := mod.RegisterCustomerUC.Execute(ctx, id, "  ", "+5531999990000")
	if nil == err {
		t.Fatal("expected Execute to fail on blank name, got nil error")
	}
	// expected the error  type to be ErrCustomerNameIsBlank
	if !errors.Is(err, domain.ErrCustomerNameIsBlank) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNameIsBlank, err)
	}
	// expected no cusotmer to have been created
	if customer != nil {
		t.Errorf("expected nil customer on error, got %+v", customer)
	}

	// try locate this cusomer in the database; should return an ErrCustomerNotFound
	if _, err := mod.CustomerRepo.FindByID(ctx, id); !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected nothing to be persisted, FindByID returned: %v", err)
	}

	// verify the number of messages in the stream; expected to be the same as before
	if after := streamMessageCount(t, ctx, mod); after != before {
		t.Errorf("expected no new message, got %d", after-before)
	}
}
