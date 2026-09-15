package application_test

import (
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestCreateShoppingList_Execute_Succeeds(t *testing.T) {

	// initialize the Module and setup a context
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// generate a shoppig list id
	id := uniqueID("create-shopping-list-test")

	// exeucte the UC for creating a shopping list
	shoppingList, err := mod.CreateShoppingListUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify if the created list id and status
	if shoppingList.ID != id {
		t.Errorf("expected shopping list ID %q, got %q", id, shoppingList.ID)
	}
	if shoppingList.Status != domain.ShoppingListPendingStatus {
		t.Errorf("expected status pending, got %s", shoppingList.Status)
	}
}
