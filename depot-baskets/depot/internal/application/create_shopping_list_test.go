package application_test

import (
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestCreateShoppingList_Execute_Succeeds(t *testing.T) {

	// initialize the Module and setup a context
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// generate a shoppig list listID
	listID := uniqueID("create-shopping-list")

	// generate a shoppig list id
	orderID := uniqueID("create-shopping-list-order")

	// exeucte the UC for creating a shopping list
	shoppingList, err := mod.CreateShoppingListUC.Execute(ctx, listID, orderID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify if the created list id and status
	if shoppingList.ID != listID {
		t.Errorf("expected shopping list ID %q, got %q", listID, shoppingList.ID)
	}

	// verify if the order it matches
	if shoppingList.OrderID != orderID {
		t.Errorf("expected order id %q, got %q", orderID, shoppingList.OrderID)
	}

	if shoppingList.Status != domain.ShoppingListPendingStatus {
		t.Errorf("expected status pending, got %s", shoppingList.Status)
	}
}
