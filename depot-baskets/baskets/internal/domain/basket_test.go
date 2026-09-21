package domain

import (
	"errors"
	"testing"
)

// newTestBasket will instantiate a Basket struct for the running
// tests.
func newTestBasket(t *testing.T) *Basket {
	t.Helper()
	basket, _, err := NewBasket("basket-1")
	if err != nil {
		t.Fatalf("failed to create test basket: %v", err)
	}

	return basket
}

func TestNewBasket(t *testing.T) {

	t.Run("valid input creates a basket and emits BasketStarted", func(t *testing.T) {

		// construct
		basket, event, err := NewBasket("basket-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if basket.ID != "basket-1" {
			t.Errorf("expected basket id basket-1, got %s", basket.ID)
		}
		if basket.Status != BasketOpenStatus {
			t.Errorf("expected new basket status open, got %s", basket.Status)
		}
		if len(basket.Items) != 0 {
			t.Errorf("expected new basket to have no items, got %+v", basket.Items)
		}

		// expected the event emited to be BasketStarted
		started, ok := event.(BasketStarted)
		if !ok {
			t.Fatalf("expected BasketStarted event, got %T", event)
		}
		if started.AggregateID() != "basket-1" {
			t.Errorf("expected aggregate id basket-1, got %s", started.AggregateID())
		}
		if started.EventName() != BasketStartedEvent {
			t.Errorf("expected event name %s, got %s", BasketStartedEvent, started.EventName())
		}
		if started.OccurredAt().IsZero() {
			t.Errorf("expected occurredAt to be set")
		}
	})
}

func TestBasket_AddItem(t *testing.T) {

	t.Run("new product adds item with quantity 1 and emits ItemAdded", func(t *testing.T) {
		// new basket
		basket := newTestBasket(t)

		// call the AddItem to add an item to receive the event
		event, err := basket.AddItem("prod-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// expected length to be 1 (1 prod added)
		if len(basket.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(basket.Items))
		}
		// expected the id to be "prod-1"
		if basket.Items[0].ProductID != "prod-1" || basket.Items[0].Quantity != 1 {
			t.Errorf("item fields not set as expected: %+v", basket.Items[0])
		}

		// expect the Event type to be ItemAdded
		added, ok := event.(ItemAdded)
		if !ok {
			t.Fatalf("expected ItemAdded event, got %T", event)
		}
		if added.ProductID != "prod-1" || added.Quantity != 1 {
			t.Errorf("event payload mismatch: %+v", added)
		}
	})

	t.Run("adding same product again increments quantity", func(t *testing.T) {

		// new Basket
		basket := newTestBasket(t)

		// adds in a product
		_, _ = basket.AddItem("prod-1")

		// add the same product
		event, err := basket.AddItem("prod-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// expect the quantity to be now 2
		if len(basket.Items) != 1 {
			t.Fatalf("expected quantity increment to not create a second item, got %d items", len(basket.Items))
		}
		if basket.Items[0].Quantity != 2 {
			t.Errorf("expected quantity 2, got %d", basket.Items[0].Quantity)
		}

		// expect the event type to be ItemAdded
		added, ok := event.(ItemAdded)
		if !ok {
			t.Fatalf("expected ItemAdded event, got %T", event)
		}
		if added.Quantity != 2 {
			t.Errorf("expected event quantity 2, got %d", added.Quantity)
		}
	})

	t.Run("blank product id returns error", func(t *testing.T) {
		basket := newTestBasket(t)

		// try adding a product with blank ID; expect to fail
		event, err := basket.AddItem("")
		if !errors.Is(err, ErrProductIDIsBlank) {
			t.Fatalf("expected ErrProductIDIsBlank, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("checked out basket returns error", func(t *testing.T) {
		// new Basket
		basket := newTestBasket(t)

		// adds in a product
		_, _ = basket.AddItem("prod-1")

		storeIDs := map[string]string{"prod-1": "store-1"}

		// go to checkout
		_, _ = basket.Checkout(storeIDs)

		// tro to add an item to an basket in the "checkout" status;
		// expect to fail
		event, err := basket.AddItem("prod-2")
		if !errors.Is(err, ErrBasketNotOpen) {
			t.Fatalf("expected ErrBasketNotOpen, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("canceled basket returns error", func(t *testing.T) {
		// new Basket
		basket := newTestBasket(t)
		// cancel the basket; should succeed at this state
		_, _ = basket.Cancel()

		// try to add a product to "canceled" basket; should fail
		event, err := basket.AddItem("prod-1")
		if !errors.Is(err, ErrBasketNotOpen) {
			t.Fatalf("expected ErrBasketNotOpen, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}

func TestBasket_RemoveItem(t *testing.T) {

	t.Run("removes item entirely when quantity is 1 and emits ItemRemoved", func(t *testing.T) {
		// new basket
		basket := newTestBasket(t)

		// adds a new item
		_, _ = basket.AddItem("prod-1")

		// removes the item; expect to succedd
		event, err := basket.RemoveItem("prod-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(basket.Items) != 0 {
			t.Errorf("expected item to be fully removed, got %+v", basket.Items)
		}

		// event type expected to be ItemRemoved
		removed, ok := event.(ItemRemoved)
		if !ok {
			t.Fatalf("expected ItemRemoved event, got %T", event)
		}
		if removed.ProductID != "prod-1" {
			t.Errorf("expected event product id prod-1, got %s", removed.ProductID)
		}
	})

	t.Run("decrements quantity when greater than 1", func(t *testing.T) {
		// new basket
		basket := newTestBasket(t)
		// add the same item twice; increases quantity
		_, _ = basket.AddItem("prod-1")
		_, _ = basket.AddItem("prod-1")

		// remove one item to receive the event
		event, err := basket.RemoveItem("prod-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(basket.Items) != 1 {
			t.Fatalf("expected item to remain with decremented quantity, got %d items", len(basket.Items))
		}
		if basket.Items[0].Quantity != 1 {
			t.Errorf("expected quantity 1, got %d", basket.Items[0].Quantity)
		}
		if event == nil {
			t.Errorf("expected ItemRemoved event, got nil")
		}
	})

	t.Run("product not in basket returns error", func(t *testing.T) {
		basket := newTestBasket(t)

		// tries to remove an item from an empty basket; should fail
		event, err := basket.RemoveItem("prod-1")
		if !errors.Is(err, ErrProductNotInBasket) {
			t.Fatalf("expected ErrProductNotInBasket, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("checked out basket returns error", func(t *testing.T) {

		// new basket
		basket := newTestBasket(t)
		// addds a new item an goes to checkout
		_, _ = basket.AddItem("prod-1")

		// pass a map of storeIDs to checkout
		storeIDs := map[string]string{"prod-1": "store-1"}

		_, _ = basket.Checkout(storeIDs)

		// try to remove an item from a basket in "chekcout" status; should fail
		event, err := basket.RemoveItem("prod-1")
		if !errors.Is(err, ErrBasketNotOpen) {
			t.Fatalf("expected ErrBasketNotOpen, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}

func TestBasket_Checkout(t *testing.T) {

	t.Run("valid checkout emits BasketCheckedOut and updates status", func(t *testing.T) {

		// new Basket
		basket := newTestBasket(t)
		// adds in an item
		_, _ = basket.AddItem("prod-1")

		storeIDs := map[string]string{"prod-1": "store-1"}

		// set the status to "checkout
		event, err := basket.Checkout(storeIDs)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// verify the event type
		if basket.Status != BasketCheckedOutStatus {
			t.Errorf("expected status checked_out, got %s", basket.Status)
		}

		checkedOut, ok := event.(BasketCheckedOut)
		if !ok {
			t.Fatalf("expected BasketCheckedOut event, got %T", event)
		}

		if checkedOut.AggregateID() != basket.ID {
			t.Errorf("expected aggregate id %s, got %s", basket.ID, checkedOut.AggregateID())
		}

		if len(checkedOut.Items) != 1 || checkedOut.Items[0].StoreID != "store-1" {
			t.Errorf("expected 1 item with store-1, got %+v", checkedOut.Items)
		}
	})

	t.Run("empty basket returns error", func(t *testing.T) {
		// new basket
		basket := newTestBasket(t)

		storeID := map[string]string{"prod-1": "store-1"}

		// try to checkout on an emtpy basket; expected to fail
		event, err := basket.Checkout(storeID)
		if !errors.Is(err, ErrBasketIsEmpty) {
			t.Fatalf("expected ErrBasketIsEmpty, got %v", err)
		}

		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}

		if basket.Status != BasketOpenStatus {
			t.Errorf("expected status to remain open, got %s", basket.Status)
		}
	})

	t.Run("already checked out returns error", func(t *testing.T) {

		basket := newTestBasket(t)
		event, _ := basket.AddItem("prod-1")

		storeIDs := map[string]string{"prod-1": "store-1"}

		// checkout the basket; basket is checkedout
		_, _ = basket.Checkout(storeIDs)

		// try checking out again
		event, err := basket.Checkout(storeIDs)
		if !errors.Is(err, ErrBasketAlreadyCheckedOut) {
			t.Fatalf("expected ErrBasketAlreadyCheckedOut, got %v", err)
		}

		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("canceled basket returns error", func(t *testing.T) {
		basket := newTestBasket(t)
		_, _ = basket.AddItem("prod-1")

		// cancel the basket
		_, _ = basket.Cancel()

		storeIDs := map[string]string{"prod-1": "store-1"}

		// try checking out a canceled basket
		event, err := basket.Checkout(storeIDs)
		if !errors.Is(err, ErrBasketNotOpen) {
			t.Fatalf("expected ErrBasketNotOpen, got %v", err)
		}

		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	t.Run("productID not in catalogue", func(t *testing.T) {
		// new basket
		basket := newTestBasket(t)

		// add an item
		_, _ = basket.AddItem("not-in-store-1")

		// pass the map of products to checkout
		storeIDs := map[string]string{"prod-from-store-1": "store-1"}

		// try checking out a product not in the store
		event, err := basket.Checkout(storeIDs)
		if !errors.Is(err, ErrProductNotInCatalog) {
			t.Fatalf("expected ErrProductNotInCatalog, got %v", err)
		}

		// expect the event not to be emited
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}

func TestBasket_Cancel(t *testing.T) {

	t.Run("valid cancel from open basket emits BasketCanceled", func(t *testing.T) {
		basket := newTestBasket(t)

		event, err := basket.Cancel()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if basket.Status != BasketCanceledStatus {
			t.Errorf("expected status canceled, got %s", basket.Status)
		}

		canceled, ok := event.(BasketCanceled)
		if !ok {
			t.Fatalf("expected BasketCanceled event, got %T", event)
		}
		if canceled.AggregateID() != basket.ID {
			t.Errorf("expected aggregate id %s, got %s", basket.ID, canceled.AggregateID())
		}
	})

	t.Run("already checked out returns error", func(t *testing.T) {

		// new basket
		basket := newTestBasket(t)

		// adds in a product
		_, _ = basket.AddItem("prod-1")

		// pass the map of products to checkout
		storeIDs := map[string]string{"prod-1": "store-1"}

		// checkout
		_, _ = basket.Checkout(storeIDs)

		// try to canceling an already checkedout basket.
		event, err := basket.Cancel()
		if !errors.Is(err, ErrBasketAlreadyCheckedOut) {
			t.Fatalf("expected ErrBasketAlreadyCheckedOut, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if basket.Status != BasketCheckedOutStatus {
			t.Errorf("expected status to remain checked_out, got %s", basket.Status)
		}
	})

	t.Run("already canceled returns error", func(t *testing.T) {
		basket := newTestBasket(t)
		_, _ = basket.Cancel()

		event, err := basket.Cancel()
		if !errors.Is(err, ErrBasketAlreadyCanceled) {
			t.Fatalf("expected ErrBasketAlreadyCanceled, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}
