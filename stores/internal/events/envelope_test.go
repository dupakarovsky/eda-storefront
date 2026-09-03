package events_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	"github.com/dupakarovsky/eda-storefront/stores/internal/events"
)

func TestEnvelope_Marshal_Unmarshal_StoreParticipationDisabled_Success(t *testing.T) {

	store, _, err := domain.NewStore("store-1", "Corner Grocer", "12 Main St")
	if err != nil {
		t.Fatalf("unexpected store error: %v", err)
	}

	if _, err := store.EnableParticipation(); err != nil {
		t.Fatalf("unexpected enable participation error: %v", err)
	}

	event, err := store.DisableParticipation()
	if err != nil {
		t.Fatalf("disable participation error: %v", err)
	}

	event, ok := event.(domain.StoreParticipationDisabled)
	if !ok {
		t.Errorf("expected StoreParticipationDisabled event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "store-1" || envelope.EventName != "StoreParticipationDisabled" {
		t.Errorf(`expected AggregateID to be "store-1" and EventName to be "StoreParticipationDisabled"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"participating":false`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var storeParticipationDisabled events.StoreParticipationDisabledPayload
	if err = json.Unmarshal(decoded.Data, &storeParticipationDisabled); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if storeParticipationDisabled.Participating != false {
		t.Error("expected Participating to be false; got true")
	}
}

func TestEnvelope_Marshal_Unmarshal_StoreRebranded_Success(t *testing.T) {

	store, _, err := domain.NewStore("store-1", "Corner Grocer", "12 Main St")
	if err != nil {
		t.Fatalf("unexpected store error: %v", err)
	}

	event, err := store.Rebrand("Downtown Grocer")
	if err != nil {
		t.Fatalf("rebrand error: %v", err)
	}

	event, ok := event.(domain.StoreRebranded)
	if !ok {
		t.Errorf("expected StoreRebranded event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "store-1" || envelope.EventName != "StoreRebranded" {
		t.Errorf(`expected AggregateID to be "store-1" and EventName to be "StoreRebranded"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"name":"Downtown Grocer"`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var storeRebranded events.StoreRebrandedPayload
	if err = json.Unmarshal(decoded.Data, &storeRebranded); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if storeRebranded.Name != "Downtown Grocer" {
		t.Errorf("expected Name to be %q; got %q", "Downtown Grocer", storeRebranded.Name)
	}
}

func TestEnvelope_Marshal_Unmarshal_ProductAdded_Success(t *testing.T) {

	_, event, err := domain.NewProduct("product-1", "store-1", "Espresso Beans", "Dark roast, 1lb bag", "SKU-001", 14.99)
	if err != nil {
		t.Fatalf("unexpected product error: %v", err)
	}

	event, ok := event.(domain.ProductAdded)
	if !ok {
		t.Errorf("expected ProductAdded event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "product-1" || envelope.EventName != "ProductAdded" {
		t.Errorf(`expected AggregateID to be "product-1" and EventName to be "ProductAdded"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"store_id":"store-1"`) || !strings.Contains(buf.String(), `"sku":"SKU-001"`) || !strings.Contains(buf.String(), `"price":14.99`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var productAdded events.ProductAddedPayload
	if err = json.Unmarshal(decoded.Data, &productAdded); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if productAdded.StoreID != "store-1" || productAdded.Name != "Espresso Beans" || productAdded.SKU != "SKU-001" || productAdded.Price != 14.99 {
		t.Errorf("expected StoreID/Name/SKU/Price to match original product; got %+v", productAdded)
	}
}

func TestEnvelope_Marshal_Unmarshal_ProductRebranded_Success(t *testing.T) {

	product, _, err := domain.NewProduct("product-1", "store-1", "Espresso Beans", "Dark roast, 1lb bag", "SKU-001", 14.99)
	if err != nil {
		t.Fatalf("unexpected product error: %v", err)
	}

	event, err := product.Rebrand("House Blend", "Medium roast, 1lb bag")
	if err != nil {
		t.Fatalf("rebrand error: %v", err)
	}

	event, ok := event.(domain.ProductRebranded)
	if !ok {
		t.Errorf("expected ProductRebranded event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "product-1" || envelope.EventName != "ProductRebranded" {
		t.Errorf(`expected AggregateID to be "product-1" and EventName to be "ProductRebranded"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"name":"House Blend"`) || !strings.Contains(buf.String(), `"description":"Medium roast, 1lb bag"`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var productRebranded events.ProductRebrandedPayload
	if err = json.Unmarshal(decoded.Data, &productRebranded); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if productRebranded.Name != "House Blend" || productRebranded.Description != "Medium roast, 1lb bag" {
		t.Errorf("expected Name/Description to match rebrand; got %+v", productRebranded)
	}
}

func TestEnvelope_Marshal_Unmarshal_ProductPriceIncreased_Success(t *testing.T) {

	product, _, err := domain.NewProduct("product-1", "store-1", "Espresso Beans", "Dark roast, 1lb bag", "SKU-001", 10.00)
	if err != nil {
		t.Fatalf("unexpected product error: %v", err)
	}

	event, err := product.IncreasePrice(12.50)
	if err != nil {
		t.Fatalf("increase price error: %v", err)
	}

	event, ok := event.(domain.ProductPriceIncreased)
	if !ok {
		t.Errorf("expected ProductPriceIncreased event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "product-1" || envelope.EventName != "ProductPriceIncreased" {
		t.Errorf(`expected AggregateID to be "product-1" and EventName to be "ProductPriceIncreased"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"delta":2.5`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var priceIncreased events.ProductPriceIncreasedPayload
	if err = json.Unmarshal(decoded.Data, &priceIncreased); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if priceIncreased.Delta != 2.50 {
		t.Errorf("expected Delta to be %v; got %v", 2.50, priceIncreased.Delta)
	}
}

func TestEnvelope_Marshal_Unmarshal_ProductPriceDecreased_Success(t *testing.T) {

	product, _, err := domain.NewProduct("product-1", "store-1", "Espresso Beans", "Dark roast, 1lb bag", "SKU-001", 10.00)
	if err != nil {
		t.Fatalf("unexpected product error: %v", err)
	}

	event, err := product.DecreasePrice(7.50)
	if err != nil {
		t.Fatalf("decrease price error: %v", err)
	}

	event, ok := event.(domain.ProductPriceDecreased)
	if !ok {
		t.Errorf("expected ProductPriceDecreased event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "product-1" || envelope.EventName != "ProductPriceDecreased" {
		t.Errorf(`expected AggregateID to be "product-1" and EventName to be "ProductPriceDecreased"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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
	if !strings.Contains(buf.String(), `"delta":-2.5`) {
		t.Error("expected Contains to return true: got false")
	}

	var decoded events.Envelope
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	var priceDecreased events.ProductPriceDecreasedPayload
	if err = json.Unmarshal(decoded.Data, &priceDecreased); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}

	if priceDecreased.Delta != -2.50 {
		t.Errorf("expected Delta to be %v; got %v", -2.50, priceDecreased.Delta)
	}
}

func TestEnvelope_Marshal_Unmarshal_ProductRemoved_Success(t *testing.T) {

	product, _, err := domain.NewProduct("product-1", "store-1", "Espresso Beans", "Dark roast, 1lb bag", "SKU-001", 10.00)
	if err != nil {
		t.Fatalf("unexpected product error: %v", err)
	}

	event, err := product.Remove()
	if err != nil {
		t.Fatalf("remove error: %v", err)
	}

	event, ok := event.(domain.ProductRemoved)
	if !ok {
		t.Errorf("expected ProductRemoved event got %T", event)
	}

	envelope, err := events.NewEnvelope(event)
	if err != nil {
		t.Fatalf("unexpected new envelope failure: %v", err)
	}

	if envelope.AggregateID != "product-1" || envelope.EventName != "ProductRemoved" {
		t.Errorf(`expected AggregateID to be "product-1" and EventName to be "ProductRemoved"; got: %s and %s`, envelope.AggregateID, envelope.EventName)
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

	var productRemoved events.ProductRemovedPayload
	if err = json.Unmarshal(decoded.Data, &productRemoved); err != nil {
		t.Fatalf("unexpected error unmarshalling: %v", err)
	}
}
