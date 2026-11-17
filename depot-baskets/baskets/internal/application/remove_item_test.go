// baskets/internal/application/remove_item_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func TestRemoveItem_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("remove-item-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	storeID := uniqueID("remove-item-store")
	productID := uniqueID("remove-item-product")
	if err := mod.CatalogRepo.CreateProduct(ctx, productID, storeID, "Eggs", "dozen, large", "SKU-EGG-1", 4.00, 0); err != nil {
		t.Fatalf("failed to seed catalog product: %v", err)
	}

	if _, err := mod.AddItemUC.Execute(ctx, basketID, productID); err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	basket, err := mod.RemoveItemUC.Execute(ctx, basketID, productID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if len(basket.Items) != 0 {
		t.Errorf("expected item to be fully removed, got %+v", basket.Items)
	}
}

func TestRemoveItem_Execute_RejectsProductNotInBasket(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	basketID := uniqueID("remove-item-reject-basket")
	if _, err := mod.StartBasketUC.Execute(ctx, basketID); err != nil {
		t.Fatalf("failed to start basket: %v", err)
	}

	_, err := mod.RemoveItemUC.Execute(ctx, basketID, uniqueID("never-added-product"))
	if err == nil {
		t.Fatal("expected Execute to fail, got nil error")
	}

	if !errors.Is(err, domain.ErrProductNotInBasket) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrProductNotInBasket, err)
	}
}
