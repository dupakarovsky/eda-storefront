package domain_test

import (
	"testing"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
)

func TestNewShoppingList(t *testing.T) {

	sl, event, err := domain.NewShoppingList("sl-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sl.OrderID != "order-1" {
		t.Errorf("expected order id %q, got %q", "order-1", sl.OrderID)
	}
	if sl.Status != domain.ShoppingListPendingStatus {
		t.Errorf("expected status %q, got %q", domain.ShoppingListPendingStatus, sl.Status)
	}
	if len(sl.Stops) != 0 {
		t.Errorf("expected empty Stops, got %d entries", len(sl.Stops))
	}

	created, ok := event.(domain.ShoppingListCreated)
	if !ok {
		t.Fatalf("expected ShoppingListCreated event, got %T", event)
	}
	if created.AggregateID() != "sl-1" {
		t.Errorf("expected aggregate id %q, got %q", "sl-1", created.AggregateID())
	}
	if created.OrderID != "order-1" {
		t.Errorf("expected event order id %q, got %q", "order-1", created.OrderID)
	}
}

func TestNewShoppingList_BlankOrderID(t *testing.T) {
	sl, event, err := domain.NewShoppingList("sl-1", "")
	if err != domain.ErrOrderIDIsBlank {
		t.Errorf("expected ErrOrderIDIsBlank, got %v", err)
	}
	if sl != nil || event != nil {
		t.Errorf("expected nil shopping list and event on error, got %+v / %+v", sl, event)
	}
}
