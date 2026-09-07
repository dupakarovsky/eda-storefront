package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestDisableStoreParticipation_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	id := "disable-test-" + time.Now().Format("20060102150405.000000")
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Disable Test Grocer", "1 Disable Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if _, err := mod.EnableStoreParticipationUC.Execute(ctx, id); err != nil {
		t.Fatalf("failed to enable participation as test setup: %v", err)
	}

	store, err := mod.DisableStoreParticipationUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if store.Participating {
		t.Error("expected store to not be participating after Execute")
	}
}

func TestDisableStoreParticipation_Execute_RejectsAlreadyDisabled(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	id := "disable-test-twice-" + time.Now().Format("20060102150405.000000")
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Disable Twice Grocer", "1 Disable Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_, err = mod.DisableStoreParticipationUC.Execute(ctx, id)
	if err == nil {
		t.Fatal("expected Execute to fail on a store that's never been enabled, got nil error")
	}

	if !errors.Is(err, domain.ErrStoreIsAlreadyNotParticipating) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStoreIsAlreadyNotParticipating, err)
	}
}
