package baskets_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/baskets"
	"github.com/nats-io/nats.go/jetstream"
)

// fakeMsg simualte a message in the jetstream that was published by "baskets"
type fakeMsg struct {
	jetstream.Msg
	data []byte
}

func (m *fakeMsg) Data() []byte {
	return m.data
}

// testItem simulates an Item that will be extracted from the
// fake jetstream message
type testItem struct {
	ProductID string `json:"product_id"`
	StoreID   string `json:"store_id"`
	Quantity  int    `json:"quantity"`
}

// testPayload simulates an BasketCheckedOut messge inside the stream
type testPayload struct {
	Items []testItem `json:"items"`
}

// testEnvelope simulate an Envelop wrapper
type testEnvelope struct {
	AggregateID string          `json:"aggregate_id"`
	EventName   string          `json:"event_name"`
	Data        json.RawMessage `json:"data"`
}

// buildCheckedOutMessage will create message similar to one that exists in
// the jestream so it can be pulled by the Handler function
func buildCheckedOutMessage(t *testing.T, items []testItem) *fakeMsg {
	t.Helper()

	// create a fake "OrderCreated" json payload
	payload, err := json.Marshal(testPayload{Items: items})
	if err != nil {
		t.Fatalf("failed to marshal test payload: %v", err)
	}

	// wrap the fake OrderCreatedPayload into a fake Envelope type
	envelope, err := json.Marshal(testEnvelope{
		AggregateID: "basket-1",
		EventName:   "BasketCheckedOut",
		Data:        payload,
	})
	if err != nil {
		t.Fatalf("failed to marshal test envelope: %v", err)
	}
	// that's the message that'll pulled from NATS by the consumer Handler

	// return the fake NATS message
	return &fakeMsg{data: envelope}
}

func TestHandler_Success(t *testing.T) {

	// create a Module struct
	mod := newTestModule(HandlerTest, t)
	ctx := t.Context()

	// define product and store IDs for the Order
	productID := uniqueID("handler-prod")
	storeID := uniqueID("handler-store")

	// create a message with an existing to simulate a published Event by the "baskets" service
	msg := buildCheckedOutMessage(t, []testItem{
		{ProductID: productID, StoreID: storeID, Quantity: 2},
	})

	// initiliaze the handler
	handler := baskets.Handler(mod.CreateOrderUC, mod.Logger)

	// call the handler: it'll call the CreateOrderUC internally which
	// will construct proper domain Order, persist to the ordering database
	// and publish the OrderCreated event into the jetstream
	if err := handler(ctx, msg); err != nil {
		t.Fatalf("expected handler to succeed, got error: %v", err)
	}

	// order and quantity variables to receive data from the ordering database
	var orderID string
	var quantity int

	// now call the database and search for the order with the data we provided;
	// should return a record
	row := mod.Pool.QueryRow(ctx, `SELECT order_id, quantity FROM order_items WHERE product_id = $1 AND store_id = $2`, productID, storeID)
	if err := row.Scan(&orderID, &quantity); err != nil {
		t.Fatalf("expected order item to be persisted, got error: %v", err)
	}

	if orderID == "" {
		t.Error("expected a non-empty order id")
	}

	if quantity != 2 {
		t.Errorf("expected quantity 2, got %d", quantity)
	}
}

func TestHandler_UnknownEventName_Skips(t *testing.T) {

	// create a module
	mod := newTestModule(HandlerTest, t)
	ctx := t.Context()

	// define proudct id bu no store id
	productID := uniqueID("handler-skip-prod")

	// build a message to simulate a basket being checkedout
	msg := buildCheckedOutMessage(t, []testItem{
		{ProductID: productID, StoreID: uniqueID("handler-skip-store"), Quantity: 1},
	})

	// now we unwrap the data from the
	var env testEnvelope
	if err := json.Unmarshal(msg.data, &env); err != nil {
		t.Fatalf("failed to unmarshal test envelope: %v", err)
	}

	// now override the EventName with an non-exsiting Event and rebuild the message
	env.EventName = "SomeOtherEvent"
	rebuilt, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("failed to remarshal test envelope: %v", err)
	}
	msg.data = rebuilt

	// instantiate the handler and  call it with that message
	handler := baskets.Handler(mod.CreateOrderUC, mod.Logger)

	// the handler should now NOT persist this event to the database.
	if err := handler(ctx, msg); err != nil {
		t.Fatalf("expected handler to skip silently, got error: %v", err)
	}

	// try querying the database; should return no rows
	var count int
	err = mod.Pool.QueryRow(ctx, `SELECT count(*) FROM order_items WHERE product_id = $1`, productID).Scan(&count)
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected no order item to be created for an unrecognized event, got %d", count)
	}
}

func TestHandler_MalformedEnvelope_ReturnsError(t *testing.T) {

	// new Module
	mod := newTestModule(HandlerTest, t)
	ctx := t.Context()

	// create a message in malformed json
	msg := &fakeMsg{data: []byte(`not-json`)}

	// initiliaze the handler
	handler := baskets.Handler(mod.CreateOrderUC, mod.Logger)

	// call the handler with malformed json; should error
	err := handler(ctx, msg)
	if err == nil {
		t.Fatal("expected handler to fail on malformed envelope, got nil error")
	}

	//
	if !strings.Contains(err.Error(), "unmarshaling envelope") {
		t.Errorf("expected error to mention envelope unmarshaling, got: %v", err)
	}
}

func TestHandler_MalformedPayload_ReturnsError(t *testing.T) {

	mod := newTestModule(HandlerTest, t)
	ctx := t.Context()

	envelope, err := json.Marshal(testEnvelope{
		AggregateID: "basket-1",
		EventName:   "BasketCheckedOut",
		Data:        json.RawMessage(`{"items":"not-an-array"}`),
	})
	if err != nil {
		t.Fatalf("failed to marshal test envelope: %v", err)
	}

	msg := &fakeMsg{data: envelope}

	handler := baskets.Handler(mod.CreateOrderUC, mod.Logger)

	err = handler(ctx, msg)
	if err == nil {
		t.Fatal("expected handler to fail on malformed payload, got nil error")
	}

	if !strings.Contains(err.Error(), "unmarshaling BasketCheckedOut payload") {
		t.Errorf("expected error to mention payload unmarshaling, got: %v", err)
	}
}
