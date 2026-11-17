// baskets/internal/application/cancel_basket_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func TestCancelBasket_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("cancel-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	basket, err := mod.CancelBasketUC.Execute(ctx, basketID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if basket.Status != domain.BasketCanceledStatus {
		t.Errorf("expected status canceled, got %s", basket.Status)
	}
}

func TestCancelBasket_Execute_RejectsAlreadyCheckedOut(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("cancel-checked-out-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	storeID := uniqueID("cancel-checked-out-store")
	productID := uniqueID("cancel-checked-out-product")
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Butter", "1 stick", "SKU-BUTTER-1", 2.00, 0); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	if _, err := mod.AddItemUC.Execute(ctx, basketID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	if _, err := mod.CheckoutUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	// try to cancel a basket after the checkout status
	_, err := mod.CancelBasketUC.Execute(ctx, basketID)
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrBasketAlreadyCheckedOut) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrBasketAlreadyCheckedOut, err)
	}
}
