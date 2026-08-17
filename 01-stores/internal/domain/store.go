package domain

import (
	"errors"
	"time"
)

// StoreCreatedEvent is the event name for creating a new store
const StoreCreatedEvent = "stores.StoreCretated"

var (
	ErrStoreNameIsBlank     = errors.New("the store name cannot be blank")
	ErrStoreLocationIsBlank = errors.New("the store location cannot be blank")
)

// Store defines the a new store, with the necessary fields, that will be offering
// products in the application
type Store struct {
	ID            string
	Name          string
	Location      string
	Participating bool
}

// NewStore is a constructor function that will instantiate a new Store
// from the values it's receives, as well as the StoreCreatedEvent when successful
func NewStore(id, name, location string, participating bool) (*Store, Event, error) {

	// evalute for empty name
	if name == "" {
		return nil, nil, ErrStoreNameIsBlank
	}

	// evalute for empty location
	if location == "" {
		return nil, nil, ErrStoreLocationIsBlank
	}

	// both checks have passed
	// -----------------------

	store := &Store{
		ID:       id,
		Name:     name,
		Location: location,
		// participating is left at zero-value: false
	}

	event := StoreCreated{
		eventBase: eventBase{aggregateID: id, occurredAt: time.Now()},
		Name:      name,
		Location:  location,
	}

	return store, event, nil

}
