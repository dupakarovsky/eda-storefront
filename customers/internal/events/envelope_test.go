package events_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	"github.com/dupakarovsky/eda-storefront/customers/internal/events"
)

// unknownEvent is a concrete Event type not mapped to the known
// Events for customers
type unknownEvent struct{}

func (unknownEvent) AggregateID() string   { return "x" }
func (unknownEvent) EventName() string     { return "Unknown" }
func (unknownEvent) OccurredAt() time.Time { return time.Now() }

// newCustomerAndEvent will create a new customer and return the CustomerRegistred Event
func newCustomerAndEvent(t *testing.T) (*domain.Customer, domain.Event) {
	t.Helper()

	// construct a new customer and receive the CustomerRegistred Event
	customer, event, err := domain.NewCustomer("customer-1", "Alice Smith", "+5531999990000")
	if err != nil {
		t.Fatalf("unexpected customer error: %v", err)
	}

	return customer, event
}

// roundTrip will perform the marshalling and unmarshalling of the Envelope type
// returning the Envelope wrapper and the payload json string
func roundTrip(t *testing.T, event domain.Event) (events.Envelope, string) {
	t.Helper()

	// create an Envelope type from Event it receives
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// convert the envelope into json
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// decoded will the decoded json data
	var decoded events.Envelope

	// decode json into the struct
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	return decoded, string(payload)
}

func TestEnvelope_Marshal_Unmarshal_CustomerRegistered_Success(t *testing.T) {

	// create a new customer and recieve the event
	_, event := newCustomerAndEvent(t)

	// get the Envelpe and the paylod data from the event
	decoded, raw := roundTrip(t, event)

	// verify the fields in the Envelope
	if decoded.AggregateID != "customer-1" || decoded.EventName != "CustomerRegistered" {
		t.Errorf(`expected "customer-1" and "CustomerRegistered"; got: %s and %s`, decoded.AggregateID, decoded.EventName)
	}
	// verify the OccurredAt fields has non-zero timestamp
	if decoded.OccurredAt.IsZero() {
		t.Errorf("expected occurred_at to survive the round trip")
	}
	// verify the payload contans the json string
	if !strings.Contains(raw, `"data":{"name":"Alice Smith","sms_number":"+5531999990000"}`) {
		t.Errorf("unexpected wire format: %s", raw)
	}

	// try unmashsing the payload into a CustomerRegisteredPayload type
	var payload events.CustomerRegisteredPayload
	if err := json.Unmarshal(decoded.Data, &payload); err != nil {
		t.Fatalf("unexpected error unmarshalling payload: %v", err)
	}
	// verify the fields
	if payload.Name != "Alice Smith" || payload.SmsNumber != "+5531999990000" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

func TestEnvelope_Marshal_Unmarshal_CustomerSmsChanged_Success(t *testing.T) {
	// new customer and event
	customer, _ := newCustomerAndEvent(t)

	// call the ChangeSmsNumber method to update the sms number
	event, err := customer.ChangeSmsNumber("+5531888880000")
	if err != nil {
		t.Fatalf("change sms number error: %v", err)
	}

	// wrap the event into an Envelpe
	decoded, raw := roundTrip(t, event)

	// verify the Envelope contains the correct values
	if decoded.AggregateID != "customer-1" || decoded.EventName != "CustomerSmsChanged" {
		t.Errorf(`expected "customer-1" and "CustomerSmsChanged"; got: %s and %s`, decoded.AggregateID, decoded.EventName)
	}

	// verify the data containes the changed sms number
	if !strings.Contains(raw, `"data":{"sms_number":"+5531888880000"}`) {
		t.Errorf("unexpected wire format: %s", raw)
	}

	// decode the payload into the CustomerSmsChangedPayload type
	var payload events.CustomerSmsChangedPayload
	if err := json.Unmarshal(decoded.Data, &payload); err != nil {
		t.Fatalf("unexpected error unmarshalling payload: %v", err)
	}
	// access the struct has the changed sms number
	if payload.SmsNumber != "+5531888880000" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}

func TestEnvelope_Marshal_Unmarshal_CustomerDisabled_Success(t *testing.T) {
	customer, _ := newCustomerAndEvent(t)

	event, err := customer.Disable()
	if err != nil {
		t.Fatalf("disable error: %v", err)
	}

	decoded, raw := roundTrip(t, event)

	if decoded.AggregateID != "customer-1" || decoded.EventName != "CustomerDisabled" {
		t.Errorf(`expected "customer-1" and "CustomerDisabled"; got: %s and %s`, decoded.AggregateID, decoded.EventName)
	}
	if !strings.Contains(raw, `"data":{}`) {
		t.Errorf(`expected data to be an empty object; got: %s`, raw)
	}

	var payload events.CustomerDisabledPayload
	if err := json.Unmarshal(decoded.Data, &payload); err != nil {
		t.Fatalf("unexpected error unmarshalling payload: %v", err)
	}
}

func TestEnvelope_Marshal_Unmarshal_CustomerEnabled_Success(t *testing.T) {
	customer, _ := newCustomerAndEvent(t)

	if _, err := customer.Disable(); err != nil {
		t.Fatalf("disable error: %v", err)
	}
	event, err := customer.Enable()
	if err != nil {
		t.Fatalf("enable error: %v", err)
	}

	decoded, raw := roundTrip(t, event)

	if decoded.AggregateID != "customer-1" || decoded.EventName != "CustomerEnabled" {
		t.Errorf(`expected "customer-1" and "CustomerEnabled"; got: %s and %s`, decoded.AggregateID, decoded.EventName)
	}
	if !strings.Contains(raw, `"data":{}`) {
		t.Errorf(`expected data to be an empty object; got: %s`, raw)
	}

	var payload events.CustomerEnabledPayload
	if err := json.Unmarshal(decoded.Data, &payload); err != nil {
		t.Fatalf("unexpected error unmarshalling payload: %v", err)
	}
}

func TestNewEnvelope_RejectsUnmappedEvent(t *testing.T) {
	// test enveloping the unknownEvent type
	_, err := events.NewEnvelope(unknownEvent{})
	if nil == err {
		t.Fatal("expected NewEnvelope to fail on an unmapped event type, got nil error")
	}
}

func TestSubjectFor(t *testing.T) {
	customer, registered := newCustomerAndEvent(t)

	changed, err := customer.ChangeSmsNumber("+5531888880000")
	if err != nil {
		t.Fatalf("change sms number error: %v", err)
	}
	disabled, err := customer.Disable()
	if err != nil {
		t.Fatalf("disable error: %v", err)
	}
	enabled, err := customer.Enable()
	if err != nil {
		t.Fatalf("enable error: %v", err)
	}

	cases := []struct {
		name  string
		event domain.Event
		want  string
	}{
		{"registered", registered, "customers.customer.CustomerRegistered"},
		{"sms changed", changed, "customers.customer.CustomerSmsChanged"},
		{"disabled", disabled, "customers.customer.CustomerDisabled"},
		{"enabled", enabled, "customers.customer.CustomerEnabled"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := events.SubjectFor(tc.event)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tc.want {
				t.Errorf("expected subject %q, got %q", tc.want, got)
			}
		})
	}

	t.Run("unmapped event returns error", func(t *testing.T) {
		if _, err := events.SubjectFor(unknownEvent{}); err == nil {
			t.Fatal("expected SubjectFor to fail on an unmapped event type, got nil error")
		}
	})
}

func TestStreamSubjects_CoverEveryPublishedSubject(t *testing.T) {
	if events.StreamName != "customers" {
		t.Errorf("expected stream name customers, got %s", events.StreamName)
	}
	if len(events.StreamSubjects) != 1 || events.StreamSubjects[0] != "customers.customer.*" {
		t.Errorf("unexpected stream subjects: %v", events.StreamSubjects)
	}
}
