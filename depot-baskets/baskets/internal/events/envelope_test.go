// baskets/internal/events/envelope_test.go
package events_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	"github.com/dupakarovsky/eda-storefront/baskets/internal/events"
)

func TestEnvelope_Marshal_Unmarshal_BasketStarted_Success(t *testing.T) {

	// generate a new basket
	_, event, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	// confirm the event is BasketStarted
	event, ok := event.(domain.BasketStarted)
	if !ok {
		t.Errorf("expected BasketStarted event got %T", event)
	}

	// envelope the Event
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "basket-1" || envelope.EventName != "BasketStarted" {
		t.Errorf(`expected AggregateID to be "basket-1" and EventName to be "BasketStarted"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// mashall the envelope into JSON
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
	// verify the json
	if !strings.Contains(buf.String(), `"data":{}`) {
		t.Error(`expected data field to be an empty object "{}"; got false`)
	}

	// decoded will receive the decoded json back
	var decoded events.Envelope
	// decode json back into the envelope struct
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// verify if the Events concrete type back into it's Payload form
	// it should be BasketStartedPayload
	var basketStarted events.BasketStartedPayload
	if err = json.Unmarshal(decoded.Data, &basketStarted); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}

func TestEnvelope_Marshal_Unmarshal_ItemAdded_Success(t *testing.T) {

	// new basket
	basket, _, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	// add in an item
	event, err := basket.AddItem("prod-1")
	if err != nil {
		t.Fatalf("add item error: %v", err)
	}

	// Event concrete type should be ItemAdded
	event, ok := event.(domain.ItemAdded)
	if !ok {
		t.Errorf("expected ItemAdded event got %T", event)
	}

	// wrape the Event type into an Envelope
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// verify the fields
	if envelope.AggregateID != "basket-1" || envelope.EventName != "ItemAdded" {
		t.Errorf(`expected AggregateID to be "basket-1" and EventName to be "ItemAdded"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// marshal the Envelope into the JSON
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// write to a buffer
	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}
	// verify the json contains the productID and quantity fields
	if !strings.Contains(buf.String(), `"product_id":"prod-1"`) || !strings.Contains(buf.String(), `"quantity":1`) {
		t.Error("expected Contains to return true: got false")
	}

	// decodee the json back to an Envelope type
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// Extract the Data field from Envelopt back to it's Payload type
	var itemAdded events.ItemAddedPayload
	if err = json.Unmarshal(decoded.Data, &itemAdded); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if itemAdded.ProductID != "prod-1" || itemAdded.Quantity != 1 {
		t.Errorf("expected ProductID/Quantity to match original item; got %+v", itemAdded)
	}
}

func TestEnvelope_Marshal_Unmarshal_ItemAdded_RunningTotal_Success(t *testing.T) {

	// new basket
	basket, _, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	// add an item
	_, err = basket.AddItem("prod-1")
	if err != nil {
		t.Fatalf("add item error: %v", err)
	}

	// and the same item; quantity should now be 2
	event, err := basket.AddItem("prod-1")
	if err != nil {
		t.Fatalf("add item error: %v", err)
	}

	// envelope the Event
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// convert into json
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// convert back into envelope
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// extract the Data type into it's proper Payload type
	var itemAdded events.ItemAddedPayload
	if err = json.Unmarshal(decoded.Data, &itemAdded); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if itemAdded.Quantity != 2 {
		t.Errorf("expected running total Quantity 2, got %d", itemAdded.Quantity)
	}
}

func TestEnvelope_Marshal_Unmarshal_ItemRemoved_Success(t *testing.T) {

	// new basket
	basket, _, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	// add in a product
	_, err = basket.AddItem("prod-1")
	if err != nil {
		t.Fatalf("add item error: %v", err)
	}

	// remove an item to recieve the event; Item should now have
	// been removed
	event, err := basket.RemoveItem("prod-1")
	if err != nil {
		t.Fatalf("remove item error: %v", err)
	}

	// verify the event type
	event, ok := event.(domain.ItemRemoved)
	if !ok {
		t.Errorf("expected ItemRemoved event got %T", event)
	}

	// envelope the envent
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	// verity the envelope fields
	if envelope.AggregateID != "basket-1" || envelope.EventName != "ItemRemoved" {
		t.Errorf(`expected AggregateID to be "basket-1" and EventName to be "ItemRemoved"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	// conver the envelope to json
	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	// write the json blob to buffer
	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}
	// verify the json integrety
	if !strings.Contains(buf.String(), `"product_id":"prod-1"`) {
		t.Error("expected Contains to return true: got false")
	}

	// conver the json back into an Envelope
	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	// decode the Data field back to it's concrete Event type
	var itemRemoved events.ItemRemovedPayload
	if err = json.Unmarshal(decoded.Data, &itemRemoved); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if itemRemoved.ProductID != "prod-1" {
		t.Errorf("expected ProductID to match original item; got %+v", itemRemoved)
	}
}

func TestEnvelope_Marshal_Unmarshal_BasketCheckedOut_Success(t *testing.T) {

	// new basket
	basket, _, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	// add an item in
	_, err = basket.AddItem("prod-1")
	if err != nil {
		t.Fatalf("add item error: %v", err)
	}

	// update the status to checkout
	event, err := basket.Checkout()
	if err != nil {
		t.Fatalf("checkout error: %v", err)
	}

	// envent type should be BasketCheckedOut
	event, ok := event.(domain.BasketCheckedOut)
	if !ok {
		t.Errorf("expected BasketCheckedOut event got %T", event)
	}

	// evenlope the event
	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "basket-1" || envelope.EventName != "BasketCheckedOut" {
		t.Errorf(`expected AggregateID to be "basket-1" and EventName to be "BasketCheckedOut"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}
	if !strings.Contains(buf.String(), `"data":{}`) {
		t.Error(`expected data field to be an empty object "{}"; got false`)
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var checkedOut events.BasketCheckedOutPayload
	if err = json.Unmarshal(decoded.Data, &checkedOut); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}

func TestEnvelope_Marshal_Unmarshal_BasketCanceled_Success(t *testing.T) {

	basket, _, err := domain.NewBasket("basket-1")
	if err != nil {
		t.Fatalf("unexpected basket error: %v", err)
	}

	event, err := basket.Cancel()
	if err != nil {
		t.Fatalf("cancel error: %v", err)
	}

	event, ok := event.(domain.BasketCanceled)
	if !ok {
		t.Errorf("expected BasketCanceled event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "basket-1" || envelope.EventName != "BasketCanceled" {
		t.Errorf(`expected AggregateID to be "basket-1" and EventName to be "BasketCanceled"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
	}

	payload, err := envelope.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshalling error: %v", err)
	}

	buf := new(bytes.Buffer)
	_, err = buf.Write(payload)
	if err != nil {
		t.Fatalf("error writing json bytes to buffer: %v", err)
	}
	if !strings.Contains(buf.String(), `"data":{}`) {
		t.Error(`expected data field to be an empty object "{}"; got false`)
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var canceled events.BasketCanceledPayload
	if err = json.Unmarshal(decoded.Data, &canceled); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}
