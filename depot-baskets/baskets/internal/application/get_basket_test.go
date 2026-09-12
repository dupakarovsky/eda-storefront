// baskets/internal/application/get_basket_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func TestGetBasket_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("get-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	basket, err := mod.GetBasketUC.Execute(ctx, basketID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if basket.ID != basketID {
		t.Errorf("expected basket ID %q, got %q", basketID, basket.ID)
	}
}

func TestGetBasket_Execute_ReturnsNotFoundForUnknownID(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	_, err := mod.GetBasketUC.Execute(ctx, uniqueID("never-created-basket"))
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrBasketNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrBasketNotFound, err)
	}
}
