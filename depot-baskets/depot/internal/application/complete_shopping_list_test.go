// depot/internal/application/complete_shopping_list_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestCompleteShoppingList_Execute_Succeeds(t *testing.T) {
	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("complete-list")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// create a store and add a product to that store
	storeID := uniqueID("complete-store")
	productID := uniqueID("complete-product")
	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Oat Milk", "1 quart", "SKU-OAT-1", 3.25); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	// add that product to the list
	if _, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	// modify the list from "pending" to "active". No more Stops can be added
	if _, err := mod.InitiateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// update the status from "active" to "complete"
	shoppingList, err := mod.CompleteShoppingListUC.Execute(ctx, listID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// check if the shopping list status is "completed"
	if shoppingList.Status != domain.ShoppingListCompletedStatus {
		t.Errorf("expected status completed, got %s", shoppingList.Status)
	}

}

func TestCompleteShoppingList_Execute_RejectsEmptyList(t *testing.T) {
	// initialize a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("complete-empty-list")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}
	// change the status to "active". No items where added
	if _, err := mod.InitiateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to initiate: %v", err)
	}

	// try completing the list ("acitve" to "completed"); should fail on an empty list
	_, err := mod.CompleteShoppingListUC.Execute(ctx, listID)
	if err == nil {
		t.Fatal("expected Execute to fail on empty shopping list, got nil error")
	}

	// verify the error type
	if !errors.Is(err, domain.ErrShoppingListIsEmpty) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrShoppingListIsEmpty, err)
	}
}

func TestCompleteShoppingList_Execute_RejectsNotActive(t *testing.T) {
	// start a Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("complete-not-active-list")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// try completing the list from the "pending" state
	_, err := mod.CompleteShoppingListUC.Execute(ctx, listID)
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	// verify the error type
	if !errors.Is(err, domain.ErrShoppingListNotActive) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrShoppingListNotActive, err)
	}
}
