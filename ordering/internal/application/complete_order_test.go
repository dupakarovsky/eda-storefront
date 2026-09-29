// ordering/internal/application/complete_order_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

func TestCompleteOrder_Execute_Succeeds(t *testing.T) {

	// create a module with context
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define a unique ID for the Order
	id := uniqueID("complete-order")

	// create the Items to this Order
	items := []domain.Item{
		{ProductID: uniqueID("complete-order-prod"), StoreID: uniqueID("complete-order-store"), Quantity: 1},
	}

	// creates a new Order. No previous Order will be found and a new one will be created,
	// saved to the orders database and the event will be published to the stream. Then the depot
	// service will be called to create a shopping list. It'll be created with this order id
	// saved to the depot's database and published the event to the steam (which order subscribes to)
	// then it'll call depot again to add the Items to the Order. Then link the ShoppingList ID to the
	// created order and update the Order in the database with this change.
	created, err := mod.CreateOrderUC.Execute(ctx, id, items)
	if err != nil {
		t.Fatalf("failed to seed order via CreateOrder: %v", err)
	}

	// after the Orders was succesfully created, the shopping list ID should now
	// be linked to the Order
	if created.ShoppingListID == "" {
		t.Fatalf("expected seeded order to carry a shopping list id, got blank")
	}

	// verify the number messages in the steram before calling the CompleteOrder
	before := streamMessageCount(t, mod, ctx)

	// call CompleteOrder to complete the order. It'll locate the Order in the
	// database by it's ShoppingListID, update it's status to "readied" and
	// save it again
	completed, err := mod.CompleteOrderUC.Execute(ctx, created.ShoppingListID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// status should now be readied
	if completed.Status != domain.OrderReadiedStatus {
		t.Errorf("expected status readied, got %s", completed.Status)
	}

	// verify the Order completed ID matches the created ID
	if completed.ID != created.ID {
		t.Errorf("expected order id %q, got %q", created.ID, completed.ID)
	}

	// The orders stream should have been updated by exaclty 1 message
	after := streamMessageCount(t, mod, ctx)
	if after != before+1 {
		t.Fatalf("expected exactly 1 new message after Execute, got %d new messages", after-before)
	}
}

func TestCompleteOrder_Execute_RejectsUnknownShoppingListID(t *testing.T) {

	// create a new Module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define an shopping list id
	unknownShoppingListID := uniqueID("never-seeded-shopping-list")

	// call the Complete method whiouth having created an Order; expect to
	// have failed to genereate an Order
	order, err := mod.CompleteOrderUC.Execute(ctx, unknownShoppingListID)
	if nil == err {
		t.Fatal("expected Execute to fail on unknown shopping list id, got nil error")
	}

	// verify if the error type is ErrOrderNotFound
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrOrderNotFound, err)
	}

	// and no order should have been created.
	if order != nil {
		t.Errorf("expected nil order on error, got %+v", order)
	}
}

func TestCompleteOrder_Execute_IsIdempotentOnRepeatedShoppingListID(t *testing.T) {

	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	id := uniqueID("complete-order-idempotent")

	items := []domain.Item{
		{ProductID: uniqueID("complete-idempotent-prod"), StoreID: uniqueID("complete-idempotent-store"), Quantity: 1},
	}

	created, err := mod.CreateOrderUC.Execute(ctx, id, items)
	if err != nil {
		t.Fatalf("failed to seed order via CreateOrder: %v", err)
	}

	first, err := mod.CompleteOrderUC.Execute(ctx, created.ShoppingListID)
	if err != nil {
		t.Fatalf("expected first Execute to succeed, got error: %v", err)
	}

	before := streamMessageCount(t, mod, ctx)

	second, err := mod.CompleteOrderUC.Execute(ctx, created.ShoppingListID)
	if err != nil {
		t.Fatalf("expected second Execute to succeed, got error: %v", err)
	}

	after := streamMessageCount(t, mod, ctx)
	if after != before {
		t.Errorf("expected no new message on repeated Execute, got %d new messages", after-before)
	}

	if second.Status != first.Status {
		t.Errorf("expected same status on repeat, got %q then %q", first.Status, second.Status)
	}

	if second.ID != first.ID {
		t.Errorf("expected same order id on repeat, got %q then %q", first.ID, second.ID)
	}
}
