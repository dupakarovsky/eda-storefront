package events_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	"github.com/dupakarovsky/eda-storefront/ordering/internal/events"
)

func TestEnvelope_Marshal_Unmarshal_OrderCreated_Success(t *testing.T) {

	// create an Item to be added to the order
	items := []domain.Item{
		{ProductID: "prod-1", StoreID: "store-1", Quantity: 2},
	}

	// generate a new order
	_, event, err := domain.NewOrder("order-1", items)
	if err != nil {
		t.Fatalf("unexpected order error: %v", err)
	}

	// check if the event type is OrderCreated
	event, ok := event.(domain.OrderCreated)
	if !ok {
		t.Errorf("expected OrderCreated event got %T", event)
	}

	// envelope the event into it's Payload type
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// verify the fields of the Envelope match
	if envelope.AggregateID != "order-1" || envelope.EventName != "OrderCreated" {
		t.Errorf(`expected AggregateID to be "order-1" and EventName to be "OrderCreated"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// convert to JSON
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// write the payload to a buffer
	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}

	// verify it has the items field
	if !strings.Contains(buf.String(), `"data":{"items":[{"product_id":"prod-1","store_id":"store-1","quantity":2}]}`) {
		t.Error("expected data field to contain the encoded items; got false")
	}

	// decode the json back into an Envelope type
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// decode the Data type back into a Payload type
	var created events.OrderCreatedPayload
	if err = json.Unmarshal(decoded.Data, &created); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	// verify if all fields match what's expected
	if len(created.Items) != 1 || created.Items[0].ProductID != "prod-1" || created.Items[0].StoreID != "store-1" || created.Items[0].Quantity != 2 {
		t.Errorf("expected item fields to match original order; got %+v", created.Items)
	}
}

func TestEnvelope_Marshal_Unmarshal_OrderReadied_Success(t *testing.T) {

	// define an Item to be added tot he order
	items := []domain.Item{
		{ProductID: "prod-1", StoreID: "store-1", Quantity: 1},
	}

	// create a new order
	order, _, err := domain.NewOrder("order-1", items)
	if err != nil {
		t.Fatalf("unexpected order error: %v", err)
	}

	// link the order to a shopping list
	if err := order.LinkShoppingList("list-1"); err != nil {
		t.Fatalf("unexpected link shopping list error: %v", err)
	}

	// mark the order as readied
	event, err := order.MarkReadied()
	if err != nil {
		t.Fatalf("mark readied error: %v", err)
	}

	// check if the event emited is OrderRedied
	event, ok := event.(domain.OrderReadied)
	if !ok {
		t.Errorf("expected OrderReadied event got %T", event)
	}

	// wrap the event in an Envelope type for transport
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// check the Envelope struct fileds match whats expected
	if envelope.AggregateID != "order-1" || envelope.EventName != "OrderReadied" {
		t.Errorf(`expected AggregateID to be "order-1" and EventName to be "OrderReadied"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// convert the Envelope to json
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// write the json to a buffer
	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}

	// the OrderReadied Event type does not have have fields, so data should be emtpy
	if !strings.Contains(buf.String(), `"data":{}`) {
		t.Error(`expected data field to be an empty object "{}"; got false`)
	}

	// decode the json back into an Envelope
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// verify the Payload type is OrderReadyPayload
	var readied events.OrderReadiedPayload
	if err = json.Unmarshal(decoded.Data, &readied); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}

func TestEnvelope_Marshal_Unmarshal_OrderCanceled_Success(t *testing.T) {

	// new Item to added to the Order
	items := []domain.Item{
		{ProductID: "prod-1", StoreID: "store-1", Quantity: 1},
	}

	// new Order
	order, _, err := domain.NewOrder("order-1", items)
	if err != nil {
		t.Fatalf("unexpected order error: %v", err)
	}

	// emit the cancel Event
	event, err := order.Cancel()
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}

	// verify the event type is OrderCanceled
	event, ok := event.(domain.OrderCanceled)
	if !ok {
		t.Errorf("expected OrderCanceled event got %T", event)
	}

	// Envelope the Event into it's proper Payload type
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// verify the fields match
	if envelope.AggregateID != "order-1" || envelope.EventName != "OrderCanceled" {
		t.Errorf(`expected AggregateID to be "order-1" and EventName to be "OrderCanceled"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// marshal the Payload into json
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// write the json to a buffer
	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}

	// the OrderCancel Event type does not have any fiedls.
	// So the Envelope Data field should be empty
	if !strings.Contains(buf.String(), `"data":{}`) {
		t.Error(`expected data field to be an empty object "{}"; got false`)
	}

	// decode the json back into an Envelope
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// verify the Payload type
	var canceled events.OrderCanceledPayload
	if err = json.Unmarshal(decoded.Data, &canceled); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}
