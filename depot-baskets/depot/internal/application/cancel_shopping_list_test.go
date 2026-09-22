// depot/internal/application/cancel_shopping_list_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestCancelShoppingList_Execute_SucceedsFromPending(t *testing.T) {
	// initiate a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("cancel-list")
	orderID := uniqueID("cancel-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// cancel the list
	shoppingList, err := mod.CancelShoppingListUC.Execute(ctx, listID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify the status is actually "canceled"
	if shoppingList.Status != domain.ShoppingListCanceledStatus {
		t.Errorf("expected status canceled, got %s", shoppingList.Status)
	}
}

func TestCancelShoppingList_Execute_RejectsAlreadyCompleted(t *testing.T) {
	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// crete a new list
	listID := uniqueID("cancel-completed-list")

	orderID := uniqueID("cancel-completed-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// create a new store and add a product in
	storeID := uniqueID("cancel-completed-store")
	productID := uniqueID("cancel-completed-product")
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Cheese", "8oz block", "SKU-CHEESE-1", 4.50); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}
	// add the product to the list
	if _, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	// initiate the list
	if _, err := mod.InitiateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}
	// complete the list
	if _, err := mod.CompleteShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	// try canceling the list when it's in the "completed" state
	_, err := mod.CancelShoppingListUC.Execute(ctx, listID)
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	// verify the error type
	if !errors.Is(err, domain.ErrShoppingListAlreadyCompleted) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrShoppingListAlreadyCompleted, err)
	}
}
