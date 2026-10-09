// customers/internal/application/change_sms_number_test.go
package application_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	customersevents "github.com/dupakarovsky/eda-storefront/customers/internal/events"
)

func TestChangeSmsNumber_Execute_Succeeds(t *testing.T) {
	// new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// seed a new cusotmer and check the message count in the stream
	seeded := registerTestCustomer(t, mod, "change-sms")
	before := streamMessageCount(t, ctx, mod)

	// update the customer's sms number; should succeed
	customer, err := mod.ChangeSmsNumberUC.Execute(ctx, seeded.ID, "+5531888880000")
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify the number has changed
	if customer.SmsNumber != "+5531888880000" {
		t.Errorf("expected returned sms number updated, got %s", customer.SmsNumber)
	}

	// query the database for the cusotmer; should succeed
	found, err := mod.CustomerRepo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}

	// verify the new number has persisted
	if found.SmsNumber != "+5531888880000" {
		t.Errorf("expected persisted sms number updated, got %s", found.SmsNumber)
	}

	// verify the number of messages in stream; should have increased by 1
	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Fatalf("expected exactly 1 new message, got %d", after-before)
	}

	// retrieve the last message
	env := lastEnvelope(t, ctx, mod, "customers.customer.CustomerSmsChanged")

	// verify the id matches the cusotmer id
	if env.AggregateID != seeded.ID {
		t.Errorf("expected last event for %s, got %+v", seeded.ID, env)
	}

	// Unmarshal the message into an the Envelope payload
	var payload customersevents.CustomerSmsChangedPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	// check the payload sms number matches the customer
	if payload.SmsNumber != "+5531888880000" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

func TestChangeSmsNumber_Execute_RejectsUnchangedNumber(t *testing.T) {
	// generate a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// add a new customer to the database
	seeded := registerTestCustomer(t, mod, "change-sms-same")
	// get the current number of messages in the stream
	before := streamMessageCount(t, ctx, mod)

	// try to modify the sms number passing the same number; should fail
	_, err := mod.ChangeSmsNumberUC.Execute(ctx, seeded.ID, seeded.SmsNumber)
	if nil == err {
		t.Fatal("expected Execute to fail on an unchanged number, got nil error")
	}

	// verify the error returned is the ErrCustomerSmsNumberUnchanged
	if !errors.Is(err, domain.ErrCustomerSmsNumberUnchanged) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerSmsNumberUnchanged, err)
	}

	// verify the stream message count remained the same
	if after := streamMessageCount(t, ctx, mod); after != before {
		t.Errorf("expected no new message, got %d", after-before)
	}
}

func TestChangeSmsNumber_Execute_RejectsUnknownCustomer(t *testing.T) {
	// generete a new Module
	mod := newTestModule(ApplicationTest, t)

	// try to change the sms number of an unknown customer; should fail
	_, err := mod.ChangeSmsNumberUC.Execute(t.Context(), uniqueID("change-sms-unknown"), "+5531888880000")
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown customer, got nil error")
	}

	// verify the error returned is ErrCustomerNotFound
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}
}
