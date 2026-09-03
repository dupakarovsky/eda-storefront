package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestGetStore_Execute_Succeeds(t *testing.T) {
	// create a new module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// set an id for the store
	id := uniqueID("get-store-test")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Get Store Grocer", "1 Get Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// execute the GetStore method; expected to succeed
	store, err := mod.GetStoreUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if store.Name != "Get Store Grocer" {
		t.Errorf("expected store name %q, got %q", "Get Store Grocer", store.Name)
	}
}

func TestGetStore_Execute_RejectsUnknownStore(t *testing.T) {

	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// try retrieving a non-existing store; expected to fail
	_, err := mod.GetStoreUC.Execute(ctx, uniqueID("get-store-unknown"))
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown store, got nil error")
	}

	// expected the error type to be ErrStoreNotFound
	if !errors.Is(err, domain.ErrStoreNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStoreNotFound, err)
	}
}
