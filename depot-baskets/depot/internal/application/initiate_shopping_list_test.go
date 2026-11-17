// depot/internal/application/initiate_shopping_list_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestInitiateShoppingList_Execute_Succeeds(t *testing.T) {

	// initialize a new Modue
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("initiate-list")
	orderID := uniqueID("initiate-order")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// creat a store and a product for that store
	storeID := uniqueID("initiate-store")
	productID := uniqueID("initiate-product")

	if err := mod.CatalogRepo.CreateStore(ctx, storeID, "Corner Grocer", "1 Main St"); err != nil {
		t.Fatalf("failed to seed catalog store: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Bread", "sourdough loaf", "SKU-BREAD-1", 5.00, 0); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}
	// add the product to the list
	if _, err := mod.AddItemUC.Execute(ctx, listID, storeID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	// initialize the shopping list: updates the status of the list and saves it.
	shoppingList, err := mod.InitiateShoppingListUC.Execute(ctx, listID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify the status changed from "pending" to "active"
	if shoppingList.Status != domain.ShoppingListActiveStatus {
		t.Errorf("expected status active, got %s", shoppingList.Status)
	}
}

func TestInitiateShoppingList_Execute_RejectsNotPending(t *testing.T) {
	// initilize a new moduel
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create a new list
	listID := uniqueID("initiate-twice-list")
	orderID := uniqueID("initiate-twice-order")

	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// try initiating a shopping list ("panding" to "active")
	if _, err := mod.InitiateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed first Initiate: %v", err)
	}

	// status should have change

	// try initiating the list again ("active" to "active"); should fail
	_, err := mod.InitiateShoppingListUC.Execute(ctx, listID)
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	// error expected
	if !errors.Is(err, domain.ErrShoppingListNotPending) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrShoppingListNotPending, err)
	}
}
