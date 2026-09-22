// depot/internal/application/add_item_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
)

func TestAddItem_Execute_Succeeds(t *testing.T) {
	// initialize the module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// generate an id for the list
	listID := uniqueID("add-item-list")

	orderID := uniqueID("add-item-list-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// generate a store and product IDs
	storeID := uniqueID("add-item-store")
	productID := uniqueID("add-item-product")

	// create the store for our testing
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	// create the product for for the store above
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Whole Milk", "1 gallon", "SKU-MILK-1", 3.50); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	// execute the AddItem to add an item to the shopping list. updates the shopping
	// list in the database
	shoppingList, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// retrive the Stop associated with this ID from the list
	stop, exists := shoppingList.Stops[storeID]
	if !exists {
		t.Fatalf("expected a stop for store %q", storeID)
	}

	// verify if the name and location matches
	if stop.StoreName != "Corner Grocer" || stop.StoreLocation != "1 Main St" {
		t.Errorf("stop store details not set as expected: %+v", stop)
	}

	if len(stop.Items) != 1 || stop.Items[0].ProductID != productID || stop.Items[0].Quantity != 1 {
		t.Errorf("item fields not set as expected: %+v", stop.Items)
	}
}

func TestAddItem_Execute_RejectsUnknownStore(t *testing.T) {
	// initialized a Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// generate a uniqueID for the list
	listID := uniqueID("add-item-unknown-store-list")
	orderID := uniqueID("add-item-unknown-store-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// generete an ids for store and product.
	// this store was needed seeded to the database
	unknownStoreID := uniqueID("never-seeded-store")
	productID := uniqueID("add-item-unknown-store-product")

	// try adding an Item to the list from the store above. expect to fail
	_, err := mod.AddItemUC.Execute(ctx, listID, unknownStoreID, productID)
	if err == nil {
		t.Fatal("expected Execute to fail on unknown store, got nil error")
	}

	// error type should be ErrStoreNotFound
	if !errors.Is(err, catalog.ErrStoreNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", catalog.ErrStoreNotFound, err)
	}
}

func TestAddItem_Execute_RejectsUnknownProduct(t *testing.T) {
	// initiate a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("add-item-unknown-product-list")
	orderID := uniqueID("add-item-unkown-product-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// create a new store
	storeID := uniqueID("add-item-unknown-product-store")
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}

	// generate an id for product. This product was not
	// seeded to the store
	unknownProductID := uniqueID("never-seeded-product")

	// try adding the product to the list; expect to fail
	_, err := mod.AddItemUC.Execute(ctx, listID, storeID, unknownProductID)
	if err == nil {
		t.Fatal("expected Execute to fail on unknown product, got nil error")
	}

	// error type should be ErrStoreNotFound
	if !errors.Is(err, catalog.ErrProductNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", catalog.ErrProductNotFound, err)
	}
}
