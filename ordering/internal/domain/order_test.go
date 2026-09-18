package domain

import (
	"errors"
	"testing"
)

// newTestOrder will instantiate an Order struct for the running
// tests.
func newTestOrder(t *testing.T) *Order {
	// mark the function as a helper
	t.Helper()

	// create a new Item for the Order to retain in a slice
	items := []Item{
		{ProductID: "prod-1", StoreID: "store-1", Quantity: 2},
	}

	// generate a new order given an ID and a slice of Items
	order, _, err := NewOrder("order-1", items)
	if err != nil {
		t.Fatalf("failed to create test order: %v", err)
	}

	return order
}

// will test the NewOrder constructor as well
func TestNewOrder(t *testing.T) {
	t.Run("valid input creates an order and emits OrderCreated", func(t *testing.T) {

		// generate a list of Items for the order
		items := []Item{
			{ProductID: "prod-1", StoreID: "store-1", Quantity: 2},
		}
		// call NewOrder giving the Order an ID and and a slice of Items
		order, event, err := NewOrder("order-1", items)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// verify the Order id
		if order.ID != "order-1" {
			t.Errorf("expected order id order-1, got %s", order.ID)
		}
		// verify the status is OrderCreated
		if order.Status != OrderCreatedStatus {
			t.Errorf("expected new order status created, got %s", order.Status)
		}
		// Items should have only 1 entry
		if len(order.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(order.Items))
		}
		// Items 1st (and only Item) ID should be "prod-1" and belong to "store-1" and have 2
		if order.Items[0].ProductID != "prod-1" || order.Items[0].StoreID != "store-1" || order.Items[0].Quantity != 2 {
			t.Errorf("item fields not set as expected: %+v", order.Items[0])
		}
		// At createion this Order's shoppingListID should be empty
		if order.ShoppingListID != "" {
			t.Errorf("expected shopping list id to be blank, got %s", order.ShoppingListID)
		}
		// verify the Event type is OrderCreated
		created, ok := event.(OrderCreated)
		if !ok {
			t.Fatalf("expected OrderCreated event, got %T", event)
		}
		// verify the aggrage ID is "order-1"
		if created.AggregateID() != "order-1" {
			t.Errorf("expected aggregate id order-1, got %s", created.AggregateID())
		}
		// verify the event name should be OrderCreatedEvent
		if created.EventName() != OrderCreatedEvent {
			t.Errorf("expected event name %s, got %s", OrderCreatedEvent, created.EventName())
		}
		// verify the time the Event was emited is now
		if created.OccurredAt().IsZero() {
			t.Errorf("expected occurredAt to be set")
		}
		if len(created.Items) != 1 || created.Items[0].ProductID != "prod-1" {
			t.Errorf("event items mismatch: %+v", created.Items)
		}
	})

	// ordering/internal/domain/order_test.go — new subtest in TestNewOrder
	t.Run("empty items returns error", func(t *testing.T) {

		// try creating a new Order with an empty slice; should fail
		order, event, err := NewOrder("order-1", []Item{})
		if !errors.Is(err, ErrOrderIsEmpty) {
			t.Fatalf("expected ErrOrderIsEmpty, got %v", err)
		}

		// no order and no event should have been emited
		if order != nil {
			t.Errorf("expected nil order on error, got %+v", order)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}

func TestOrder_LinkShoppingList(t *testing.T) {

	t.Run("valid link sets the shopping list id", func(t *testing.T) {

		// gerate an Order
		order := newTestOrder(t)

		// attacha shopping list to this Order
		err := order.LinkShoppingList("list-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// verify that the Order now has th ShoppingListID filled it
		if order.ShoppingListID != "list-1" {
			t.Errorf("expected shopping list id list-1, got %s", order.ShoppingListID)
		}
	})

	t.Run("blank shopping list id returns error", func(t *testing.T) {
		// create an order
		order := newTestOrder(t)

		// try adding a blank shopping list ID to the Order; should fail
		err := order.LinkShoppingList("")
		if !errors.Is(err, ErrShoppingListIDIsBlank) {
			t.Fatalf("expected ErrShoppingListIDIsBlank, got %v", err)
		}

		// ShoppignListID property should still be empty
		if order.ShoppingListID != "" {
			t.Errorf("expected shopping list id to remain blank, got %s", order.ShoppingListID)
		}
	})

	t.Run("already linked returns error", func(t *testing.T) {
		// creat a new Order
		order := newTestOrder(t)
		// link a shopping list ID to this Order
		_ = order.LinkShoppingList("list-1")

		// try linking another shopping list ID to this Order; an Order
		// can be linked to only 1 shopping list
		err := order.LinkShoppingList("list-2")
		if !errors.Is(err, ErrShoppingListAlreadyLinked) {
			t.Fatalf("expected ErrShoppingListAlreadyLinked, got %v", err)
		}
		// verify it's linked to 'list-1'
		if order.ShoppingListID != "list-1" {
			t.Errorf("expected shopping list id to remain list-1, got %s", order.ShoppingListID)
		}
	})
}

func TestOrder_MarkReadied(t *testing.T) {

	t.Run("valid mark readied emits OrderReadied and updates status", func(t *testing.T) {
		// generate a new Order
		order := newTestOrder(t)

		// link this order to shopping list ID "list-1"
		_ = order.LinkShoppingList("list-1")

		// change the state of the order to "readied"
		event, err := order.MarkReadied()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// verify if ths status has changed
		if order.Status != OrderReadiedStatus {
			t.Errorf("expected status readied, got %s", order.Status)
		}

		// verify the Event type is OrderReadied
		readied, ok := event.(OrderReadied)
		if !ok {
			t.Fatalf("expected OrderReadied event, got %T", event)
		}

		// check if the Event aggregateID matches the order id
		if readied.AggregateID() != order.ID {
			t.Errorf("expected aggregate id %s, got %s", order.ID, readied.AggregateID())
		}
	})

	t.Run("already readied returns error", func(t *testing.T) {
		// new Order
		order := newTestOrder(t)

		// link e shopping list before marking it ready
		_ = order.LinkShoppingList("list-1")

		// mark the Order as "readied"
		_, _ = order.MarkReadied()

		// try marking it again; should fail
		event, err := order.MarkReadied()
		if !errors.Is(err, ErrOrderAlreadyReadied) {
			t.Fatalf("expected ErrOrderAlreadyReadied, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("already canceled returns error", func(t *testing.T) {
		// new Order
		order := newTestOrder(t)

		// cancel the order
		_, _ = order.Cancel()

		// try chaning the status from "canceled" to "readied"; shoulf fail
		event, err := order.MarkReadied()
		if !errors.Is(err, ErrOrderAlreadyCanceled) {
			t.Fatalf("expected ErrOrderAlreadyCanceled, got %v", err)
		}

		// verify no Event type was emited this time
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}

		// verify the status is still "canceled"
		if order.Status != OrderCanceledStatus {
			t.Errorf("expected status to remain canceled, got %s", order.Status)
		}
	})

	//
	t.Run("not yet linked to a shopping list returns error", func(t *testing.T) {
		// new Order
		order := newTestOrder(t)

		// mark the order as "readied" without linking it to a shopping list id; should fail
		event, err := order.MarkReadied()
		if !errors.Is(err, ErrShoppingListNotLinked) {
			t.Fatalf("expected ErrShoppingListNotLinked, got %v", err)
		}

		// verify not Event was emited
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		// order status should still be "created"
		if order.Status != OrderCreatedStatus {
			t.Errorf("expected status to remain created, got %s", order.Status)
		}
	})
}

func TestOrder_Cancel(t *testing.T) {

	t.Run("valid cancel from created emits OrderCanceled", func(t *testing.T) {

		// new Order
		order := newTestOrder(t)

		// cancel the Order and ge the OrderCanceled Event
		event, err := order.Cancel()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// verify the status is "canceled"
		if order.Status != OrderCanceledStatus {
			t.Errorf("expected status canceled, got %s", order.Status)
		}

		// check the Event type is OrderCanceled
		canceled, ok := event.(OrderCanceled)
		if !ok {
			t.Fatalf("expected OrderCanceled event, got %T", event)
		}
		// check if aggregateID in the Event matches the order id
		if canceled.AggregateID() != order.ID {
			t.Errorf("expected aggregate id %s, got %s", order.ID, canceled.AggregateID())
		}
	})

	t.Run("already readied returns error", func(t *testing.T) {

		// new Order
		order := newTestOrder(t)

		// link this order to shopping list ID "list-1"
		_ = order.LinkShoppingList("list-1")

		// marked the Order as "readied"
		_, _ = order.MarkReadied()

		// try canceling the Order after "readied"; should fail
		event, err := order.Cancel()
		if !errors.Is(err, ErrOrderAlreadyReadied) {
			t.Fatalf("expected ErrOrderAlreadyReadied, got %v", err)
		}

		// verify no Event was emited this time
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		// status should still be OrderReadiedStatus
		if order.Status != OrderReadiedStatus {
			t.Errorf("expected status to remain readied, got %s", order.Status)
		}
	})

	t.Run("already canceled returns error", func(t *testing.T) {

		// new Order
		order := newTestOrder(t)

		// cancel the Order
		_, _ = order.Cancel()

		// try canceling a second time; should fail
		event, err := order.Cancel()
		if !errors.Is(err, ErrOrderAlreadyCanceled) {
			t.Fatalf("expected ErrOrderAlreadyCanceled, got %v", err)
		}

		// no Event should have been emited a this time
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}
