package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// loadRealEnv will need to load the .env
func TestEnableStoreParticipation_Execute_Succeeds(t *testing.T) {

	mod := newTestModule(t)

	// context for the test to run
	ctx := t.Context()

	// define a unique ID for the Store to be created
	id := "enable-test-" + time.Now().Format("20060102150405.000000")

	// try to create the Store with the publishRetry; expected to succeed
	store, err := mod.CreateStoreUC.Execute(ctx, id, "Enable Test Grocer", "1 Enable Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// verify if the created store id matches our provided id
	if store.ID != id {
		t.Errorf("expected store ID %q, got %q", id, store.ID)
	}

	// run execute use case for enabling participation; expected to succeed
	store, err = mod.EnableStoreParticipationUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected EnableStoreParticipationUC to succeed; failed: %v", err)
	}

	// expected store to be participating
	if !store.Participating {
		t.Errorf("expected Participating to be true; go false")
	}
}

func TestEnableStoreParticipation_Execute_RejectsAlreadyEnabled(t *testing.T) {
	mod := newTestModule(t)
	ctx := t.Context()

	id := "enable-twice-test-" + time.Now().Format("20060102150405.000000")
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Enable Twice Grocer", "1 Enable Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if _, err := mod.EnableStoreParticipationUC.Execute(ctx, id); err != nil {
		t.Fatalf("failed on first enable: %v", err)
	}

	_, err = mod.EnableStoreParticipationUC.Execute(ctx, id)
	if err == nil {
		t.Fatal("expected second Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrStoreIsAlreadyParticipating) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStoreIsAlreadyParticipating, err)
	}
}
