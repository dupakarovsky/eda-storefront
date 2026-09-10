package domain

import (
	"errors"
	"time"
)

// Sentinel errors for the Basket aggregate
var (
	ErrProductIDIsBlank        = errors.New("the product id cannot be blank")
	ErrProductNotInBasket      = errors.New("the product is not in the basket")
	ErrBasketNotOpen           = errors.New("the basket is not open")
	ErrBasketAlreadyCheckedOut = errors.New("the basket is already checked out")
	ErrBasketAlreadyCanceled   = errors.New("the basket is already canceled")
	ErrBasketIsEmpty           = errors.New("the basket has no items")
)

// BasketStatus defines the possible status for a give Basket
type BasketStatus string

const (
	BasketOpenStatus       BasketStatus = "open"
	BasketCheckedOutStatus BasketStatus = "checked_out"
	BasketCanceledStatus   BasketStatus = "canceled"
)

// Item defines a product added to the basket
type Item struct {
	ProductID string
	Quantity  int
}

// Basket representes a running baske tin the system, composes of a unique ID
// a status a colleciton of Item entries
type Basket struct {
	ID     string
	Status BasketStatus
	Items  []Item
}

// NewBasket instantitate a new Basket from a provided id, returning
// a BasketCreated event
func NewBasket(id string) (*Basket, Event, error) {

	// instantitate a new basket, setting the default Status to "open"
	basket := &Basket{
		ID:     id,
		Status: BasketOpenStatus,
	}

	// emit a new basket created event
	event := BasketStarted{
		eventBase: eventBase{aggregateID: id, occurredAt: time.Now()},
	}

	return basket, event, nil
}

// AddItem will add a new item to the Basket's slice and emit an
// event
func (b *Basket) AddItem(productID string) (Event, error) {

	// guard against blank entries
	if productID == "" {
		return nil, ErrProductIDIsBlank
	}

	// guard against adding a new product to an a different status
	// than "open"
	if b.Status != BasketOpenStatus {
		return nil, ErrBasketNotOpen
	}

	// from here, item can be safely added
	//-------------------------------------

	// loop for checking if the productID is alreayd in the Item list
	for i := range b.Items {

		// verify if this product in the basket already.
		if b.Items[i].ProductID == productID {

			// If so, increment it's quantity
			b.Items[i].Quantity++

			// generate a new the ItemAdded concrete  type and return here
			return ItemAdded{
				eventBase: eventBase{aggregateID: b.ID, occurredAt: time.Now()},
				ProductID: productID,
				Quantity:  b.Items[i].Quantity,
			}, nil

		}
	}
	// passed here this product was not yet added
	// ------------------------------------------

	// create the Item to be added in,starting with 1
	newItem := Item{
		ProductID: productID,
		Quantity:  1,
	}

	// update the slice with the new item
	b.Items = append(b.Items, newItem)

	// return the concrete Event type
	return ItemAdded{
		eventBase: eventBase{aggregateID: b.ID, occurredAt: time.Now()},
		ProductID: productID,
		Quantity:  1,
	}, nil

}

// RemoveItem will remove an Item from the Basket's slice and emit an
// ItemRemoved event
func (b *Basket) RemoveItem(productID string) (Event, error) {

	// guard against adding a new product to an a different status
	// than "open"
	if b.Status != BasketOpenStatus {
		return nil, ErrBasketNotOpen
	}

	// loop over all items in the slice to check if the productID
	// is from any of the Items
	for i := range b.Items {
		// found a match:
		if b.Items[i].ProductID == productID {
			// check this Item's quantity: if it's the last item or not
			switch quantity := b.Items[i].Quantity; quantity > 1 {
			// quantity > 1: decrement
			case true:
				b.Items[i].Quantity--
			// last item: remove from item
			default:
				// clear the slice for this Item
				b.Items = append(b.Items[:i], b.Items[i+1:]...)
			}

			// emit the ItemRemove concrete Event type
			return ItemRemoved{
				eventBase: eventBase{aggregateID: b.ID, occurredAt: time.Now()},
				ProductID: productID,
			}, nil
		}
	}

	// loop completed: not matches found
	// ----------------------------------

	// emit the error
	return nil, ErrProductNotInBasket
}

// Checkout will shift the state of the Basket from "open" to "checkout"
func (b *Basket) Checkout() (Event, error) {

	// check if the current status is "open"
	if b.Status != BasketOpenStatus {
		return nil, ErrBasketNotOpen
	}

	// basket is in open status
	// ------------------------

	// check if there are products in the basket
	if len(b.Items) == 0 {
		return nil, ErrBasketIsEmpty
	}

	// basket has products
	// -------------------

	// update the status to "Checkout"
	b.Status = BasketCheckedOutStatus

	// emit the Event type
	return BasketCheckedOut{
		eventBase: eventBase{aggregateID: b.ID, occurredAt: time.Now()},
	}, nil
}

// Cancel will cancel an existing Basket and chage it's status
func (b *Basket) Cancel() (Event, error) {

	// Basket status cannot be in a "checkout" stutus to proceed
	if b.Status == BasketCheckedOutStatus {
		return nil, ErrBasketAlreadyCheckedOut
	}

	// Basket status cannot be in a "cancel" status to proceed
	if b.Status == BasketCanceledStatus {
		return nil, ErrBasketAlreadyCanceled
	}

	// passed here Basket can be cancelled
	// -----------------------------------

	// update the status
	b.Status = BasketCanceledStatus

	// emit the concrete Event type
	return BasketCanceled{
		eventBase: eventBase{aggregateID: b.ID, occurredAt: time.Now()},
	}, nil
}

// ReconstituteBasket will construct a Basket struct from the data
// returned by the database
func ReconstituteBasket(id string, status BasketStatus, items []Item) *Basket {
	return &Basket{
		ID:     id,
		Status: status,
		Items:  items,
	}
}
