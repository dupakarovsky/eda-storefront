// customers/internal/domain/customer_test.go
package domain

import (
	"errors"
	"testing"
)

func newTestCustomer(t *testing.T) *Customer {
	t.Helper()

	customer, _, err := NewCustomer("customer-1", "Alice Smith", "+5531999990000")
	if err != nil {
		t.Fatalf("failed to create test customer: %v", err)
	}

	return customer
}

func TestNewCustomer(t *testing.T) {

	t.Run("valid input creates an enabled customer and emits CustomerRegistered", func(t *testing.T) {
		customer, event, err := NewCustomer("customer-1", "Alice Smith", "+5531999990000")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if customer.ID != "customer-1" || customer.Name != "Alice Smith" || customer.SmsNumber != "+5531999990000" {
			t.Errorf("customer fields not set as expected: %+v", customer)
		}
		if !customer.Enabled {
			t.Errorf("expected new customer to be enabled by default")
		}

		registered, ok := event.(CustomerRegistered)
		if !ok {
			t.Fatalf("expected CustomerRegistered event, got %T", event)
		}
		if registered.AggregateID() != "customer-1" {
			t.Errorf("expected aggregate id customer-1, got %s", registered.AggregateID())
		}
		if registered.EventName() != CustomerRegisteredEvent {
			t.Errorf("expected event name %s, got %s", CustomerRegisteredEvent, registered.EventName())
		}
		if registered.Name != "Alice Smith" || registered.SmsNumber != "+5531999990000" {
			t.Errorf("event payload mismatch: %+v", registered)
		}
		if registered.OccurredAt().IsZero() {
			t.Errorf("expected occurredAt to be set")
		}
	})

	t.Run("surrounding whitespace is trimmed in both the aggregate and the event", func(t *testing.T) {
		customer, event, err := NewCustomer("customer-1", "  Alice Smith  ", "  +5531999990000 ")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if customer.Name != "Alice Smith" || customer.SmsNumber != "+5531999990000" {
			t.Errorf("expected trimmed customer fields, got %+v", customer)
		}

		registered := event.(CustomerRegistered)
		if registered.Name != "Alice Smith" || registered.SmsNumber != "+5531999990000" {
			t.Errorf("expected trimmed event payload, got %+v", registered)
		}
	})

	t.Run("blank name returns error", func(t *testing.T) {
		customer, event, err := NewCustomer("customer-1", "", "+5531999990000")
		if !errors.Is(err, ErrCustomerNameIsBlank) {
			t.Fatalf("expected ErrCustomerNameIsBlank, got %v", err)
		}
		if customer != nil || event != nil {
			t.Errorf("expected nil customer and event on error")
		}
	})

	t.Run("whitespace-only name returns error", func(t *testing.T) {
		customer, event, err := NewCustomer("customer-1", "   ", "+5531999990000")
		if !errors.Is(err, ErrCustomerNameIsBlank) {
			t.Fatalf("expected ErrCustomerNameIsBlank, got %v", err)
		}
		if customer != nil || event != nil {
			t.Errorf("expected nil customer and event on error")
		}
	})

	t.Run("blank sms number returns error", func(t *testing.T) {
		customer, event, err := NewCustomer("customer-1", "Alice Smith", "")
		if !errors.Is(err, ErrCustomerSmsNumberIsBlank) {
			t.Fatalf("expected ErrCustomerSmsNumberIsBlank, got %v", err)
		}
		if customer != nil || event != nil {
			t.Errorf("expected nil customer and event on error")
		}
	})

	t.Run("whitespace-only sms number returns error", func(t *testing.T) {
		_, _, err := NewCustomer("customer-1", "Alice Smith", "  \t ")
		if !errors.Is(err, ErrCustomerSmsNumberIsBlank) {
			t.Fatalf("expected ErrCustomerSmsNumberIsBlank, got %v", err)
		}
	})

	t.Run("blank name is reported before a blank sms number", func(t *testing.T) {
		_, _, err := NewCustomer("customer-1", "", "")
		if !errors.Is(err, ErrCustomerNameIsBlank) {
			t.Fatalf("expected ErrCustomerNameIsBlank, got %v", err)
		}
	})
}

func TestCustomer_ChangeSmsNumber(t *testing.T) {

	t.Run("new number updates the customer and emits CustomerSmsChanged", func(t *testing.T) {
		customer := newTestCustomer(t)

		event, err := customer.ChangeSmsNumber("+5531888880000")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if customer.SmsNumber != "+5531888880000" {
			t.Errorf("expected sms number updated, got %s", customer.SmsNumber)
		}

		changed, ok := event.(CustomerSmsChanged)
		if !ok {
			t.Fatalf("expected CustomerSmsChanged, got %T", event)
		}
		if changed.AggregateID() != "customer-1" {
			t.Errorf("expected aggregate id customer-1, got %s", changed.AggregateID())
		}
		if changed.EventName() != CustomerSmsChangedEvent {
			t.Errorf("expected event name %s, got %s", CustomerSmsChangedEvent, changed.EventName())
		}
		if changed.SmsNumber != "+5531888880000" {
			t.Errorf("event payload mismatch: %+v", changed)
		}
		if changed.OccurredAt().IsZero() {
			t.Errorf("expected occurredAt to be set")
		}
	})

	t.Run("surrounding whitespace is trimmed in both the aggregate and the event", func(t *testing.T) {
		customer := newTestCustomer(t)

		event, err := customer.ChangeSmsNumber("  +5531888880000  ")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if customer.SmsNumber != "+5531888880000" {
			t.Errorf("expected trimmed sms number, got %q", customer.SmsNumber)
		}
		if event.(CustomerSmsChanged).SmsNumber != "+5531888880000" {
			t.Errorf("expected trimmed event payload, got %q", event.(CustomerSmsChanged).SmsNumber)
		}
	})

	t.Run("blank number fails and does not mutate customer", func(t *testing.T) {
		customer := newTestCustomer(t)
		original := customer.SmsNumber

		event, err := customer.ChangeSmsNumber("   ")
		if !errors.Is(err, ErrCustomerSmsNumberIsBlank) {
			t.Fatalf("expected ErrCustomerSmsNumberIsBlank, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if customer.SmsNumber != original {
			t.Errorf("expected sms number unchanged, got %s", customer.SmsNumber)
		}
	})

	t.Run("same number fails and emits no event", func(t *testing.T) {
		customer := newTestCustomer(t)

		event, err := customer.ChangeSmsNumber(customer.SmsNumber)
		if !errors.Is(err, ErrCustomerSmsNumberUnchanged) {
			t.Fatalf("expected ErrCustomerSmsNumberUnchanged, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("same number with surrounding whitespace is still unchanged", func(t *testing.T) {
		customer := newTestCustomer(t)

		_, err := customer.ChangeSmsNumber("  " + customer.SmsNumber + "  ")
		if !errors.Is(err, ErrCustomerSmsNumberUnchanged) {
			t.Fatalf("expected ErrCustomerSmsNumberUnchanged, got %v", err)
		}
	})
}

func TestCustomer_Disable(t *testing.T) {

	t.Run("enabled customer is disabled and emits CustomerDisabled", func(t *testing.T) {
		customer := newTestCustomer(t)

		event, err := customer.Disable()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if customer.Enabled {
			t.Errorf("expected customer to be disabled")
		}

		disabled, ok := event.(CustomerDisabled)
		if !ok {
			t.Fatalf("expected CustomerDisabled, got %T", event)
		}
		if disabled.AggregateID() != "customer-1" {
			t.Errorf("expected aggregate id customer-1, got %s", disabled.AggregateID())
		}
		if disabled.EventName() != CustomerDisabledEvent {
			t.Errorf("expected event name %s, got %s", CustomerDisabledEvent, disabled.EventName())
		}
	})

	t.Run("already disabled returns error and emits no event", func(t *testing.T) {
		customer := newTestCustomer(t)
		_, _ = customer.Disable()

		event, err := customer.Disable()
		if !errors.Is(err, ErrCustomerAlreadyDisabled) {
			t.Fatalf("expected ErrCustomerAlreadyDisabled, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if customer.Enabled {
			t.Errorf("expected customer to remain disabled")
		}
	})
}

func TestCustomer_Enable(t *testing.T) {

	t.Run("disabled customer is enabled and emits CustomerEnabled", func(t *testing.T) {
		customer := newTestCustomer(t)
		_, _ = customer.Disable()

		event, err := customer.Enable()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !customer.Enabled {
			t.Errorf("expected customer to be enabled")
		}

		enabled, ok := event.(CustomerEnabled)
		if !ok {
			t.Fatalf("expected CustomerEnabled, got %T", event)
		}
		if enabled.AggregateID() != "customer-1" {
			t.Errorf("expected aggregate id customer-1, got %s", enabled.AggregateID())
		}
		if enabled.EventName() != CustomerEnabledEvent {
			t.Errorf("expected event name %s, got %s", CustomerEnabledEvent, enabled.EventName())
		}
	})

	t.Run("already enabled returns error and emits no event", func(t *testing.T) {
		customer := newTestCustomer(t)

		event, err := customer.Enable()
		if !errors.Is(err, ErrCustomerAlreadyEnabled) {
			t.Fatalf("expected ErrCustomerAlreadyEnabled, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if !customer.Enabled {
			t.Errorf("expected customer to remain enabled")
		}
	})
}

func TestCustomer_Authorize(t *testing.T) {

	t.Run("enabled customer is authorized", func(t *testing.T) {
		customer := newTestCustomer(t)

		if err := customer.Authorize(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("disabled customer is not authorized", func(t *testing.T) {
		customer := newTestCustomer(t)
		_, _ = customer.Disable()

		err := customer.Authorize()
		if !errors.Is(err, ErrCustomerNotAuthorized) {
			t.Fatalf("expected ErrCustomerNotAuthorized, got %v", err)
		}
	})

	t.Run("re-enabled customer is authorized again", func(t *testing.T) {
		customer := newTestCustomer(t)
		_, _ = customer.Disable()
		_, _ = customer.Enable()

		if err := customer.Authorize(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("authorize does not mutate the customer", func(t *testing.T) {
		customer := newTestCustomer(t)
		before := *customer

		_ = customer.Authorize()

		if *customer != before {
			t.Errorf("expected customer unchanged, before %+v after %+v", before, *customer)
		}
	})
}

func TestReconstituteCustomer(t *testing.T) {

	t.Run("restores every field exactly, including a disabled state", func(t *testing.T) {
		customer := ReconstituteCustomer("customer-9", "Bob Jones", "+5531777770000", false)

		if customer.ID != "customer-9" || customer.Name != "Bob Jones" || customer.SmsNumber != "+5531777770000" {
			t.Errorf("customer fields not restored as expected: %+v", customer)
		}
		if customer.Enabled {
			t.Errorf("expected customer to be restored as disabled")
		}
	})
}
