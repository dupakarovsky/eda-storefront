// depot/internal/application/get_shopping_list_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestGetShoppingList_Execute_Succeeds(t *testing.T) {
	// initilize a Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// crete a new list
	listID := uniqueID("get-list")
	if _, err := mod.CreateShoppingListUC.Execute(ctx, listID); err != nil {
		t.Fatalf("failed to create shopping list: %v", err)
	}

	// retrieve the list from the database
	shoppingList, err := mod.GetShoppingListUC.Execute(ctx, listID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if shoppingList.ID != listID {
		t.Errorf("expected shopping list ID %q, got %q", listID, shoppingList.ID)
	}
}

func TestGetShoppingList_Execute_ReturnsNotFoundForUnknownID(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	_, err := mod.GetShoppingListUC.Execute(ctx, uniqueID("never-created-list"))
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrShoppingListNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrShoppingListNotFound, err)
	}
}
