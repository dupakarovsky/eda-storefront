// baskets/internal/application/checkout_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func TestCheckout_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("checkout-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	storeID := uniqueID("checkout-store")
	productID := uniqueID("checkout-product")
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Bread", "sourdough loaf", "SKU-BREAD-1", 5.00); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	if _, err := mod.AddItemUC.Execute(ctx, basketID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	basket, err := mod.CheckoutUC.Execute(ctx, basketID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if basket.Status != domain.BasketCheckedOutStatus {
		t.Errorf("expected status checked_out, got %s", basket.Status)
	}
}

func TestCheckout_Execute_RejectsEmptyBasket(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("checkout-empty-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	// try to checkout an empty basket
	_, err := mod.CheckoutUC.Execute(ctx, basketID)
	if err == nil {
		t.Fatal("expected Execute to fail on empty basket, got nil error")
	}

	if !errors.Is(err, domain.ErrBasketIsEmpty) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrBasketIsEmpty, err)
	}
}
