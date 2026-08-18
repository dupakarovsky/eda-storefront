package domain

import (
	"errors"
	"time"
)

var (
	ErrStoreNameIsBlank               = errors.New("the store name cannot be blank")
	ErrStoreLocationIsBlank           = errors.New("the store location cannot be blank")
	ErrStoreIsAlreadyParticipating    = errors.New("the store is already participating")
	ErrStoreIsAlreadyNotParticipating = errors.New("the store is already not participating")
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
func NewStore(id, name, location string) (*Store, Event, error) {

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

// EnableParticipation enables a Store in participating in the application
// and emits a new StoreParticipationEnabled Event
func (s *Store) EnableParticipation() (Event, error) {
	// errors when Participating is already true
	if s.Participating {
		return nil, ErrStoreIsAlreadyParticipating
	}

	// updates the local property
	s.Participating = true

	// emit a new Event
	return StoreParticipationEnabled{
		eventBase:     eventBase{aggregateID: s.ID, occurredAt: time.Now()},
		Participating: true,
	}, nil
}

// DisablesParticipation removes a Store from participating in the application
// and emits a new Store Event
func (s *Store) DisableParticipation() (Event, error) {
	if !s.Participating {
		return nil, ErrStoreIsAlreadyNotParticipating
	}

	// updates the local property
	s.Participating = false

	// emit a new Event
	return StoreParticipationDisabled{
		eventBase:     eventBase{aggregateID: s.ID, occurredAt: time.Now()},
		Participating: false,
	}, nil

}

// Rerand will modify a Stores name and emit a StoreReabarnd event
func (s *Store) Rebrand(name string) (Event, error) {
	// emit error when empty
	if name == "" {
		return nil, ErrStoreNameIsBlank
	}

	// Update the Name property
	s.Name = name

	// emit a new Event
	return StoreRebranded{
		eventBase: eventBase{aggregateID: s.ID, occurredAt: time.Now()},
		Name:      name,
	}, nil

}
