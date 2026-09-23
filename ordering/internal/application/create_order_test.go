package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
)

func TestCreateOrder_Execute_Succeeds(t *testing.T) {

	// create a new module and the context for the test
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// create an unique ID for this order
	id := uniqueID("create-order")

	// defina an slice of Items for the Order to contain
	items := []domain.Item{
		{ProductID: uniqueID("create-order-prod"), StoreID: uniqueID("create-order-store"), Quantity: 2},
	}

	// call the CreateOrder use case: should create an order, save to the orderingdb database
	// and publish the CreateOrder event to the stream
	order, err := mod.CreateOrderUC.Execute(ctx, id, items)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// verify the Order ID field matches
	if order.ID != id {
		t.Errorf("expected order id %q, got %q", id, order.ID)
	}

	// verify the status is OrderCreated
	if order.Status != domain.OrderCreatedStatus {
		t.Errorf("expected status created, got %s", order.Status)
	}

	// in a brand new Order the ShoppingListID should still be blank
	if order.ShoppingListID != "" {
		t.Errorf("expected shopping list id to be blank, got %s", order.ShoppingListID)
	}

	//verify that the order Items contain the product id, store id and quantity expected
	if len(order.Items) != 1 || order.Items[0].ProductID != items[0].ProductID || order.Items[0].StoreID != items[0].StoreID || order.Items[0].Quantity != 2 {
		t.Errorf("item fields not set as expected: %+v", order.Items)
	}
}

func TestCreateOrder_Execute_RejectsEmptyItems(t *testing.T) {

	// new module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// unique ID for the order
	id := uniqueID("create-order-empty")

	// try creating an Order with an empty Item slice; should fail
	order, err := mod.CreateOrderUC.Execute(ctx, id, []domain.Item{})
	if err == nil {
		t.Fatal("expected Execute to fail on empty items, got nil error")
	}

	// verify the error type
	if !errors.Is(err, domain.ErrOrderIsEmpty) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrOrderIsEmpty, err)
	}

	if order != nil {
		t.Errorf("expected nil order on error, got %+v", order)
	}
}
