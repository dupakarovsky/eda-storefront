package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestRebrandStore_Succeeds(t *testing.T) {

	// instantiate the module and the context for the test
	mod := newTestModule(t)
	ctx := t.Context()

	// unique ID for this test
	id := "rebrand-test-" + time.Now().Format("20060102150405.000000")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Original Name", "1 Original Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// call the RebrandStoreUC; expected to succeed;
	store, err := mod.RebrandStoreUC.Execute(ctx, id, "New Name")
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// expected the name to have changed
	if store.Name != "New Name" {
		t.Errorf("expected store name %q, got %q", "New Name", store.Name)
	}
}

func TestRebrandStore_RejectsBlankName(t *testing.T) {

	// instantiate the module and the context for the test
	mod := newTestModule(t)
	ctx := t.Context()

	// unique ID for this test
	id := "rebrand-blank-test-" + time.Now().Format("20060102150405.000000")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Original Name", "1 Original Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// call the RebrandStoreUC with blank name; expected to fail;
	_, err = mod.RebrandStoreUC.Execute(ctx, id, "")
	if nil == err {
		t.Fatal("expected Execute to fail on blank name, got nil error")
	}

	// expected the name to have changed
	if !errors.Is(err, domain.ErrStoreNameIsBlank) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStoreNameIsBlank, err)
	}
}
