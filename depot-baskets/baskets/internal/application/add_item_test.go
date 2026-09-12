// baskets/internal/application/add_item_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
)

func TestAddItem_Execute_Succeeds(t *testing.T) {
	// start the test module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// starts a new basket
	basketID := uniqueID("add-item-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	// some seed data for store
	storeID := uniqueID("add-item-store")
	productID := uniqueID("add-item-product")

	// generate data for the catalog_store database
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Whole Milk", "1 gallon", "SKU-MILK-1", 3.50); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	// add an item from the store to the basket
	basket, err := mod.AddItemUC.Execute(ctx, basketID, productID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if len(basket.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(basket.Items))
	}
	if basket.Items[0].ProductID != productID || basket.Items[0].Quantity != 1 {
		t.Errorf("item fields not set as expected: %+v", basket.Items[0])
	}
}

func TestAddItem_Execute_RejectsUnknownProduct(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// generate a new basket
	basketID := uniqueID("add-item-unknown-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	unknownProductID := uniqueID("never-seeded-product")

	// try to add a product that is not in the database
	_, err := mod.AddItemUC.Execute(ctx, basketID, unknownProductID)
	if err == nil {
		t.Fatal("expected Execute to fail on unknown product, got nil error")
	}

	if !errors.Is(err, catalog.ErrProductNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", catalog.ErrProductNotFound, err)
	}
}
