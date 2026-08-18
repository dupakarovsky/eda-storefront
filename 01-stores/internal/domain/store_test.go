package domain

import (
	"errors"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, _, err := NewStore("store-1", "Joe's Pizza", "123 Main St")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return store
}

func TestNewStore(t *testing.T) {

	// 1st Test: Valid Input
	t.Run("valid input creates a store and emits StoreCreated", func(t *testing.T) {
		store, event, err := NewStore("store-1", "Joe's Pizza", "123 Main St")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if store.ID != "store-1" || store.Name != "Joe's Pizza" || store.Location != "123 Main St" {
			t.Errorf("store fields not set as expected: %+v", store)
		}
		if store.Participating {
			t.Errorf("expected new store to not be participating by default")
		}

		created, ok := event.(StoreCreated)
		if !ok {
			t.Fatalf("expected StoreCreated event, got %T", event)
		}
		if created.AggregateID() != "store-1" {
			t.Errorf("expected aggregate id store-1, got %s", created.AggregateID())
		}
		if created.EventName() != StoreCreatedEvent {
			t.Errorf("expected event name %s, got %s", StoreCreatedEvent, created.EventName())
		}
		if created.Name != "Joe's Pizza" || created.Location != "123 Main St" {
			t.Errorf("event payload mismatch: %+v", created)
		}
		if created.OccurredAt().IsZero() {
			t.Errorf("expected occurredAt to be set")
		}
	})

	// 2nd Test: Blank store name
	t.Run("blank name returns error", func(t *testing.T) {
		// invoke the NewStore function with a blank Store name
		store, event, err := NewStore("store-1", "", "123 Main St")

		// expected to fail with an ErrStoreNameIsBlank
		if !errors.Is(err, ErrStoreNameIsBlank) {
			t.Fatalf("expected ErrStoreNameIsBlank, got %v", err)
		}

		// expected no Store to be constructed
		if store != nil {
			t.Errorf("expected nil store on error, got %+v", store)
		}

		// expected to Event to be emited
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})

	// 3rd Test: No Address provided
	t.Run("blank location returns error", func(t *testing.T) {
		// invoke the NewStore function with no Location. Expected to fail
		_, _, err := NewStore("store-1", "Joe's Pizza", "")
		if !errors.Is(err, ErrStoreLocationIsBlank) {
			t.Fatalf("expected ErrStoreLocationIsBlank, got %v", err)
		}
	})
}

func TestStore_EnableParticipation(t *testing.T) {
	// 1st Test: Enabling participation
	t.Run("enables participation and emits StoreParticipationEnable", func(t *testing.T) {

		// invoke the helper to receive a constructed Store
		store := newTestStore(t)

		// invoke EnableParticipation. Expected to succeed.
		event, err := store.EnableParticipation()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// expected the Participating property to be true
		if !store.Participating {
			t.Errorf("expected store to be participating")
		}

		// expected the event type to be StoreParticpationEnabled
		enabled, ok := event.(StoreParticipationEnabled)
		if !ok {
			t.Fatalf("expected StoreParticipationEnabled, got %T", event)
		}

		// expected the Participating property to be true now
		if !enabled.Participating {
			t.Errorf("expected Participating true in event payload")
		}

		// expected the event name to be StoreParticipationEnabledEvent
		if enabled.EventName() != StoreParticipationEnabledEvent {
			t.Errorf("expected event name %s, got %s", StoreParticipationEnabledEvent, enabled.EventName())
		}
	})

	// 2nd Test:
	t.Run("already participating returns error", func(t *testing.T) {
		// create a new Store
		store := newTestStore(t)

		// and enable participation
		_, _ = store.EnableParticipation()

		// call EnableParticipation again; Expected to fail
		event, err := store.EnableParticipation()
		if !errors.Is(err, ErrStoreIsAlreadyParticipating) {
			t.Fatalf(">>> expected ErrStoreIsAlreadyParticipating, got %v", err)
		}

		// expected to create no events
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}

		// expected Participating to be true
		if !store.Participating {
			t.Errorf("expected store to remain participating")
		}
	})
}

func TestStore_DisableParticipation(t *testing.T) {

	// 1st Test: Success
	t.Run("disables participation and emits StoreParticipationDisabled", func(t *testing.T) {

		// creates a new store
		store := newTestStore(t)

		// then enable participation
		_, _ = store.EnableParticipation()

		// now disable participation. Expected to succeed
		event, err := store.DisableParticipation()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// expected Participation to be false
		if store.Participating {
			t.Errorf("expected store to not be participating")
		}

		// expected the type of event to be StoreParticipationDisabled
		disabled, ok := event.(StoreParticipationDisabled)
		if !ok {
			t.Fatalf("expected StoreParticipationDisabled, got %T", event)
		}

		if disabled.Participating {
			t.Errorf("expected Participating false in event payload")
		}
	})

	// 2nd Test: Error
	t.Run("already not participating returns error", func(t *testing.T) {
		// instantiate a new Store
		store := newTestStore(t)

		// Expected to fail
		event, err := store.DisableParticipation()
		if !errors.Is(err, ErrStoreIsAlreadyNotParticipating) {
			t.Fatalf("expected ErrStoreIsAlreadyNotParticipating, got %v", err)
		}

		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
	})
}

func TestStore_Rebrand(t *testing.T) {

	// 1st Test: Success
	t.Run("valid name rebrands and emits StoreRebranded", func(t *testing.T) {
		// new store
		store := newTestStore(t)

		// rebrand the store; Expecte to succeed
		event, err := store.Rebrand("New Name")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if store.Name != "New Name" {
			t.Errorf("expected store name to be updated, got %s", store.Name)
		}

		// event type expected to be StoreRebranded
		rebranded, ok := event.(StoreRebranded)
		if !ok {
			t.Fatalf("expected StoreRebranded, got %T", event)
		}
		if rebranded.Name != "New Name" {
			t.Errorf("expected event Name New Name, got %s", rebranded.Name)
		}
	})

	// 2nd Test: Failure
	t.Run("blank name returns error and does not mutate store", func(t *testing.T) {
		store := newTestStore(t)
		originalName := store.Name

		event, err := store.Rebrand("")
		if !errors.Is(err, ErrStoreNameIsBlank) {
			t.Fatalf("expected ErrStoreNameIsBlank, got %v", err)
		}

		// expected no event created
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}

		// expected the store name to be unchanged
		if store.Name != originalName {
			t.Errorf("expected store name unchanged, got %s", store.Name)
		}
	})
}
