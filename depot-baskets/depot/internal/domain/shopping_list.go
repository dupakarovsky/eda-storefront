// depot/internal/domain/shopping_list.go
package domain

import (
	"errors"
	"time"
)

var (
	ErrStoreIDIsBlank               = errors.New("the store id cannot be blank")
	ErrProductIDIsBlank             = errors.New("the product id cannot be blank")
	ErrStopNotFound                 = errors.New("the store has no stop in this shopping list")
	ErrProductNotInStop             = errors.New("the product is not in this store's stop")
	ErrShoppingListNotPending       = errors.New("the shopping list is not pending")
	ErrShoppingListNotActive        = errors.New("the shopping list is not active")
	ErrShoppingListIsEmpty          = errors.New("the shopping list has no items")
	ErrShoppingListAlreadyCompleted = errors.New("the shopping list is already completed")
	ErrShoppingListAlreadyCanceled  = errors.New("the shopping list is already canceled")
)

// Item defines an Item from a Store
type Item struct {
	ProductID string
	Quantity  int
}

// Stop is physical Store where to stop to collect an Item in th
// Shopping List
type Stop struct {
	StoreName     string
	StoreLocation string
	Items         []Item
}

// ShoppingList defines the type tha aggragates all the Stop needed
// to collect Items from a different stores in a Shopping List
type ShoppingList struct {
	ID     string
	Status ShoppingListStatus
	Stops  map[string]*Stop
}

// NewShoppingList will instantiate a new shopping list
func NewShoppingList(id string) (*ShoppingList, Event, error) {

	// instantiate a new ShoppingList
	shoppingList := &ShoppingList{
		ID:     id,
		Status: ShoppingListPendingStatus,
		Stops:  make(map[string]*Stop), // iniialize the map of Stops
	}

	// emit the Event type
	event := ShoppingListCreated{
		eventBase: eventBase{aggregateID: id, occurredAt: time.Now()},
	}

	return shoppingList, event, nil
}

// AddItem adds an Item to an existsing Shopping List
func (sl *ShoppingList) AddItem(storeID, storeName, storeLocation, productID string) (Event, error) {

	// guard agains status "pending"
	if sl.Status != ShoppingListPendingStatus {
		return nil, ErrShoppingListNotPending
	}

	// storeID needs to be provided
	if storeID == "" {
		return nil, ErrStoreIDIsBlank
	}

	// productID needs to be provided
	if productID == "" {
		return nil, ErrProductIDIsBlank
	}

	// check if this store is already in the Stops map
	stop, exists := sl.Stops[storeID]
	if !exists {
		// if not, then add a new Stop keying by the storeID
		stop = &Stop{
			StoreName:     storeName,
			StoreLocation: storeLocation,
		}
		sl.Stops[storeID] = stop
	}

	// stop exists or was created; time add Items to it
	// -------------------------------------------------

	// range over all current Items from this store (checks if this product
	// was already added)
	for i := range stop.Items {

		// the product from this store is already in this list
		if stop.Items[i].ProductID == productID {

			// then update/increment the quantity of this product
			stop.Items[i].Quantity++

			// Emit the ItemAdded event
			return ItemAdded{
				eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
				StoreID:   storeID,
				ProductID: productID,
				Quantity:  stop.Items[i].Quantity,
			}, nil
		}
	}

	// passed here the product ID was in this Stop Items
	// --------------------------------------------------

	// create a new Item and add it to the Stop's Items field
	stop.Items = append(stop.Items, Item{ProductID: productID, Quantity: 1})

	// how return the ItemAdded Event type with quantity of 1
	return ItemAdded{
		eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
		StoreID:   storeID,
		ProductID: productID,
		Quantity:  1,
	}, nil
}

// RemoveItem removes an Item to an existsing Shopping List Stop
func (sl *ShoppingList) RemoveItem(storeID, productID string) (Event, error) {

	// guard against "pending" status
	if sl.Status != ShoppingListPendingStatus {
		return nil, ErrShoppingListNotPending
	}

	// verify first if this store ID is of any Stop in the ShoppingList
	// (we can only remove Items from a Stop in the list)
	stop, exists := sl.Stops[storeID]
	if !exists {
		return nil, ErrStopNotFound
	}

	// a Stop was found with this storeID
	// ----------------------------------

	// range over all Items in this Stop
	for i := range stop.Items {

		// Item with this product has matched
		if stop.Items[i].ProductID == productID {
			// verify if the quantity is above 1
			switch quantity := stop.Items[i].Quantity; quantity > 1 {
			case true:
				// if so, remove 1
				stop.Items[i].Quantity--

			// it's equal to one: then remove the Item from the Items
			default:
				stop.Items = append(stop.Items[:i], stop.Items[i+1:]...)
			}

			// if this Stop has now zero Items after the previous removal
			// the just simply this Stop from the Stops list
			if len(stop.Items) == 0 {
				delete(sl.Stops, storeID)
			}

			// emit the ItemRemoved Event

			return ItemRemoved{
				eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
				StoreID:   storeID,
				ProductID: productID,
			}, nil
		}
	}

	// if we passed this point, then this productID is not in the
	// Stop provided. return the error

	return nil, ErrProductNotInStop
}

// Initiate will update the status of a Shopping List to Active
func (sl *ShoppingList) Initiate() (Event, error) {

	// list must be on "pending" to then become Active
	if sl.Status != ShoppingListPendingStatus {
		return nil, ErrShoppingListNotPending
	}

	// update the stauts to "active""
	sl.Status = ShoppingListActiveStatus

	// return a ShoppingListInitiated Event
	return ShoppingListInitiated{
		eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
	}, nil
}

// Complete will update the status of a Shopping List to Completed
func (sl *ShoppingList) Complete() (Event, error) {

	// list must be "active" in order move to "complete"
	if sl.Status != ShoppingListActiveStatus {
		return nil, ErrShoppingListNotActive
	}

	// guard against empty list. Cannot the complete with zero items
	if len(sl.Stops) == 0 {
		return nil, ErrShoppingListIsEmpty
	}

	// update the status
	sl.Status = ShoppingListCompletedStatus

	// emit the Event
	return ShoppingListCompleted{
		eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
	}, nil
}

// Cancel will update the stauts of the Shopping List to Canceled
func (sl *ShoppingList) Cancel() (Event, error) {

	if sl.Status == ShoppingListCompletedStatus {
		return nil, ErrShoppingListAlreadyCompleted
	}

	if sl.Status == ShoppingListCanceledStatus {
		return nil, ErrShoppingListAlreadyCanceled
	}

	sl.Status = ShoppingListCanceledStatus

	return ShoppingListCanceled{
		eventBase: eventBase{aggregateID: sl.ID, occurredAt: time.Now()},
	}, nil
}

// ReconstituteShoppingList will convert data from the database into ShipponList struct
func ReconstituteShoppingList(id string, status ShoppingListStatus, stops map[string]*Stop) *ShoppingList {
	return &ShoppingList{
		ID:     id,
		Status: status,
		Stops:  stops,
	}
}
