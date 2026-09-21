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

	// ---------------------------------------------------------------------------
	// -- Add 2 products to the database, one for each store ---
	storeID1 := uniqueID("exp-checkout-store-1")
	productID1 := uniqueID("exp-checkout-product-1")

	storeID2 := uniqueID("exp-checkout-store-2")
	productID2 := uniqueID("exp-checkout-product-2")

	if err := mod.CatalogRepo.CreateProduct(ctx, productID1, storeID1, "Bread", "sourdough loaf", "SKU-BREAD-1", 5.00); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}
	if err := mod.CatalogRepo.CreateProduct(ctx, productID2, storeID2, "Milk", "whole milk", "SKU-MILK-1", 2.95); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}
	// ---------------------------------------------------------------------------

	// add product 1 to the basket
	if _, err := mod.AddItemUC.Execute(ctx, basketID, productID1); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	// add product 2 to the basket
	if _, err := mod.AddItemUC.Execute(ctx, basketID, productID2); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	//
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
