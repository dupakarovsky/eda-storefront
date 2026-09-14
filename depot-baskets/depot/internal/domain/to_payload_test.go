package domain_test

import (
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	"github.com/dupakarovsky/eda-storefront/depot/internal/events"
)

func TestShoppingList_toPayload(t *testing.T) {

	sl, _, _ := domain.NewShoppingList("sl-1")
	event1, _ := sl.AddItem("store-1", "Store A", "Avenue ALetter, number 123", "product-1")
	// event2, _ := sl.AddItem("store-1", "Store A", "Avenue ALetter, number 123", "product-2")

	envelope1, err := events.NewEnvelope(event1)
	if err != nil {
		t.Fatalf("error creating envelopping: %v", err)
	}

	env1Json, err := envelope1.Marshal()
	if err != nil {
		t.Fatalf("error marshaling envelopping: %v", err)
	}

	// json [
	// {
	// 	"aggregate_id":"sl-1",
	// 	"event_name":"ItemAdded",
	// 	"occurred_at":"2026-09-14T15:41:34.405014322-03:00",
	// 	"data":{
	// 		"store_id":"store-1",
	// 		"product_id":"product-1",
	// 		"quantity":1}
	// 	}
	// ]

	t.Logf("json [%v]", string(env1Json))
}
