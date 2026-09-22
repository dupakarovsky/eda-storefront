package domain

import (
	"testing"
)

func TestNewShoppingList(t *testing.T) {

	// new list
	sl, event, err := NewShoppingList("sl-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// initial status should be "pending"
	if sl.Status != ShoppingListPendingStatus {
		t.Errorf("expected status %q, got %q", ShoppingListPendingStatus, sl.Status)
	}
	if len(sl.Stops) != 0 {
		t.Errorf("expected empty Stops, got %d entries", len(sl.Stops))
	}

	// emit the Event
	created, ok := event.(ShoppingListCreated)
	if !ok {
		t.Fatalf("expected ShoppingListCreated event, got %T", event)
	}
	if created.AggregateID() != "sl-1" {
		t.Errorf("expected aggregate id %q, got %q", "sl-1", created.AggregateID())
	}
}

func TestShoppingList_AddItem_BlankStoreID(t *testing.T) {
	// new shopping list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add an item that is not blank
	_, err := sl.AddItem("", "Store A", "1 Main St", "product-1")
	if err != ErrStoreIDIsBlank {
		t.Errorf("expected ErrStoreIDIsBlank, got %v", err)
	}
}

func TestShoppingList_AddItem_BlankProductID(t *testing.T) {
	// new shopping list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// try adding an blank item
	_, err := sl.AddItem("store-1", "Store A", "1 Main St", "")
	if err != ErrProductIDIsBlank {
		t.Errorf("expected ErrProductIDIsBlank, got %v", err)
	}
}

func TestShoppingList_AddItem_NotPending(t *testing.T) {
	// new shopping list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// manually update the the status to "active". now it should not be possible to add more
	// items to the list
	sl.Status = ShoppingListActiveStatus

	// try adding an new item; should fail
	_, err := sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	if err != ErrShoppingListNotPending {
		t.Errorf("expected ErrShoppingListNotPending, got %v", err)
	}
}

func TestShoppingList_AddItem_NewStop(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add a new item
	event, err := sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	//  verify that we have "stop-1" added to Stops
	stop, exists := sl.Stops["store-1"]
	if !exists {
		t.Fatalf("expected a stop for store-1")
	}
	if stop.StoreName != "Store A" || stop.StoreLocation != "1 Main St" {
		t.Errorf("stop store details mismatch: got %q / %q", stop.StoreName, stop.StoreLocation)
	}
	if len(stop.Items) != 1 || stop.Items[0].ProductID != "product-1" || stop.Items[0].Quantity != 1 {
		t.Errorf("expected one item at quantity 1, got %+v", stop.Items)
	}

	// verify thet event is ItemAdded
	added, ok := event.(ItemAdded)
	if !ok {
		t.Fatalf("expected ItemAdded event, got %T", event)
	}
	if added.StoreID != "store-1" || added.ProductID != "product-1" || added.Quantity != 1 {
		t.Errorf("unexpected event fields: %+v", added)
	}
}

func TestShoppingList_AddItem_ExistingStopNewProduct(t *testing.T) {

	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add a new item to the list under "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// add another item under "store-1"
	_, err := sl.AddItem("store-1", "Store A", "1 Main St", "product-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we have two entires under the "store-1" Stop
	stop := sl.Stops["store-1"]
	if len(stop.Items) != 2 {
		t.Errorf("expected 2 items in stop, got %d", len(stop.Items))
	}
}

func TestShoppingList_AddItem_RepeatProductIncrementsQuantity(t *testing.T) {

	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add a new item under "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// verify the the same item under "store-1". Qauntity of "product-1" should now be 2
	event, err := sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// get the "store-1" stop
	stop := sl.Stops["store-1"]
	if len(stop.Items) != 1 {
		t.Errorf("expected quantity merge, not a new item; got %d items", len(stop.Items))
	}

	// verify the "product-1" quantity is actually 2
	if stop.Items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", stop.Items[0].Quantity)
	}

	// get the Event and verify the Quantity field
	added := event.(ItemAdded)
	if added.Quantity != 2 {
		t.Errorf("expected event quantity 2 (running total), got %d", added.Quantity)
	}
}

func TestShoppingList_AddItem_SecondStore(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add "product-1" under "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// add the same code but for "store-2" (both stores have a product with the same code)
	sl.AddItem("store-2", "Store B", "2 Oak St", "product-1")

	// veirfy we have two Stops
	if len(sl.Stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(sl.Stops))
	}

	// verify each Stop has each one "product-1""
	if sl.Stops["store-1"].Items[0].Quantity != 1 || sl.Stops["store-2"].Items[0].Quantity != 1 {
		t.Errorf("expected the same product id under two separate stops to be tracked independently")
	}
}

func TestShoppingList_RemoveItem_NotPending(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add an Item to "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.Status = ShoppingListActiveStatus

	// remove that Item from "store-1"
	_, err := sl.RemoveItem("store-1", "product-1")
	if err != ErrShoppingListNotPending {
		t.Errorf("expected ErrShoppingListNotPending, got %v", err)
	}
}

func TestShoppingList_RemoveItem_StopNotFound(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// try removing an non-existing Item from the Stop "store-1"
	_, err := sl.RemoveItem("store-1", "product-1")
	if err != ErrStopNotFound {
		t.Errorf("expected ErrStopNotFound, got %v", err)
	}
}

func TestShoppingList_RemoveItem_ProductNotInStop(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add "product-1" to Stop "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// try removing "product-2" from "store-1"; should error
	_, err := sl.RemoveItem("store-1", "product-2")
	if err != ErrProductNotInStop {
		t.Errorf("expected ErrProductNotInStop, got %v", err)
	}
}

func TestShoppingList_RemoveItem_Decrements(t *testing.T) {

	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add  "product-1" two times to "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// reomve one Item from "store-1"; should now have only 1
	event, err := sl.RemoveItem("store-1", "product-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// get the Stop "store-1"; should have 1 Item with 1 quantity
	stop := sl.Stops["store-1"]
	if len(stop.Items) != 1 || stop.Items[0].Quantity != 1 {
		t.Errorf("expected quantity to drop to 1, got %+v", stop.Items)
	}

	// verify the event type is ItemRemoved
	removed := event.(ItemRemoved)
	if removed.StoreID != "store-1" || removed.ProductID != "product-1" {
		t.Errorf("unexpected event fields: %+v", removed)
	}
}

func TestShoppingList_RemoveItem_LastItemDeletesStop(t *testing.T) {

	// new item
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add "product-1" to "store-1"
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	// remove "product-1" to "store-1"; no products left. Stop "store-1" will
	// will also be removed
	_, err := sl.RemoveItem("store-1", "product-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify that "store-1" Stop was removed
	if _, exists := sl.Stops["store-1"]; exists {
		t.Errorf("expected stop for store-1 to be deleted once its last item was removed")
	}
}

func TestShoppingList_RemoveItem_SameProductDifferentStores(t *testing.T) {
	// new list
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	// add "product-1" from "store-1" and "product-1" from "store-2"
	// both stores have a product with the same code
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.AddItem("store-2", "Store B", "2 Oak St", "product-1")

	// remove the "store-1" Item "product-1"; Stop "store-1" should have
	// been removed from the List
	_, err := sl.RemoveItem("store-1", "product-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify "stop-1" was removed;
	if _, exists := sl.Stops["store-1"]; exists {
		t.Errorf("expected store-1's stop to be gone")
	}

	// "store-2" should still be present
	if _, exists := sl.Stops["store-2"]; !exists {
		t.Errorf("expected store-2's stop to be untouched")
	}
}

func TestShoppingList_Initiate_NotPending(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.Status = ShoppingListActiveStatus

	_, err := sl.Initiate()
	if err != ErrShoppingListNotPending {
		t.Errorf("expected ErrShoppingListNotPending, got %v", err)
	}
}

func TestShoppingList_Initiate_Success(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")

	event, err := sl.Initiate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sl.Status != ShoppingListActiveStatus {
		t.Errorf("expected status %q, got %q", ShoppingListActiveStatus, sl.Status)
	}
	if _, ok := event.(ShoppingListInitiated); !ok {
		t.Fatalf("expected ShoppingListInitiated event, got %T", event)
	}
}

func TestShoppingList_AddItem_BlockedAfterInitiate(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.Initiate()

	_, err := sl.AddItem("store-1", "Store A", "1 Main St", "product-2")
	if err != ErrShoppingListNotPending {
		t.Errorf("expected AddItem to be blocked once active, got %v", err)
	}
}

func TestShoppingList_Complete_NotActive(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	_, err := sl.Complete()
	if err != ErrShoppingListNotActive {
		t.Errorf("expected ErrShoppingListNotActive, got %v", err)
	}
}

func TestShoppingList_Complete_EmptyStops(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.Initiate()

	_, err := sl.Complete()
	if err != ErrShoppingListIsEmpty {
		t.Errorf("expected ErrShoppingListIsEmpty, got %v", err)
	}
}

func TestShoppingList_Complete_Success(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.Initiate()

	event, err := sl.Complete()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sl.Status != ShoppingListCompletedStatus {
		t.Errorf("expected status %q, got %q", ShoppingListCompletedStatus, sl.Status)
	}
	if _, ok := event.(ShoppingListCompleted); !ok {
		t.Fatalf("expected ShoppingListCompleted event, got %T", event)
	}
}

func TestShoppingList_Cancel_FromPending(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")

	event, err := sl.Cancel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sl.Status != ShoppingListCanceledStatus {
		t.Errorf("expected status %q, got %q", ShoppingListCanceledStatus, sl.Status)
	}
	if _, ok := event.(ShoppingListCanceled); !ok {
		t.Fatalf("expected ShoppingListCanceled event, got %T", event)
	}
}

func TestShoppingList_Cancel_FromActive(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.Initiate()

	_, err := sl.Cancel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sl.Status != ShoppingListCanceledStatus {
		t.Errorf("expected status %q, got %q", ShoppingListCanceledStatus, sl.Status)
	}
}

func TestShoppingList_Cancel_AlreadyCompleted(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.AddItem("store-1", "Store A", "1 Main St", "product-1")
	sl.Initiate()
	sl.Complete()

	_, err := sl.Cancel()
	if err != ErrShoppingListAlreadyCompleted {
		t.Errorf("expected ErrShoppingListAlreadyCompleted, got %v", err)
	}
}

func TestShoppingList_Cancel_AlreadyCanceled(t *testing.T) {
	sl, _, _ := NewShoppingList("sl-1", "order-1")
	sl.Cancel()

	_, err := sl.Cancel()
	if err != ErrShoppingListAlreadyCanceled {
		t.Errorf("expected ErrShoppingListAlreadyCanceled, got %v", err)
	}
}
