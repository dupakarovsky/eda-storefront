// depot/internal/application/remove_item_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestRemoveItem_Execute_Succeeds(t *testing.T) {
	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("remove-item-list")
	orderID := uniqueID("remove-item-order")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// create a new store and product
	storeID := uniqueID("remove-item-store")
	productID := uniqueID("remove-item-product")
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Eggs", "dozen, large", "SKU-EGG-1", 4.00, 0); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	// add the product to the list
	if _, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	// now remove that product from the list
	shoppingList, err := mod.RemoveItemUC.Execute(ctx, listID, storeID, productID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// when the only product is removed, the Stop should have been removed
	if _, exists := shoppingList.Stops[storeID]; exists {
		t.Errorf("expected stop to be removed once its last item was removed, got %+v", shoppingList.Stops)
	}
}

func TestRemoveItem_Execute_RejectsStopNotFound(t *testing.T) {

	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("remove-item-no-stop-list")
	orderID := uniqueID("remove-item-no-stop-order")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// list has no Stops
	// try removing an product from a store that was not created
	_, err := mod.RemoveItemUC.Execute(ctx, listID, uniqueID("never-added-store"), uniqueID("never-added-product"))
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	// should returned shold be ErrStopNotFound
	if !errors.Is(err, domain.ErrStopNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStopNotFound, err)
	}
}

func TestRemoveItem_Execute_RejectsProductNotInStop(t *testing.T) {

	// initialize a Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("remove-item-wrong-product-list")
	orderID := uniqueID("remove-item-wrong-product-order")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// create a store a product for that store
	storeID := uniqueID("remove-item-wrong-product-store")
	productID := uniqueID("remove-item-wrong-product-seeded")
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Butter", "1 stick", "SKU-BUTTER-1", 2.00, 0); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	// add the product to the list
	if _, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	// try removing a product that was neved added to the store
	_, err := mod.RemoveItemUC.Execute(ctx, listID, storeID, uniqueID("never-added-product"))
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrProductNotInStop) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrProductNotInStop, err)
	}
}
