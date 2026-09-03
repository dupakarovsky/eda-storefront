// 01-stores/internal/application/decrease_product_price_test.go
package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestDecreaseProductPrice_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("decrease-price-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Decrease Price Grocer", "1 Decrease Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	productID := uniqueID("decrease-price-test")
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Eggs", "dozen, large", "SKU-DEC-1", 6.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	product, err := mod.DecreaseProductPriceUC.Execute(ctx, productID, 5.00)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if product.Price != 5.00 {
		t.Errorf("expected price %v, got %v", 5.00, product.Price)
	}
}

func TestDecreaseProductPrice_Execute_RejectsNonDecrease(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("decrease-price-reject-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Decrease Price Reject Grocer", "1 Decrease Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	productID := uniqueID("decrease-price-reject-test")
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Eggs", "dozen, large", "SKU-DEC-2", 6.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	_, err = mod.DecreaseProductPriceUC.Execute(ctx, productID, 6.00)
	if err == nil {
		t.Fatal("expected Execute to fail on an equal target price, got nil error")
	}

	if !errors.Is(err, domain.ErrNotAPriceDecrease) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrNotAPriceDecrease, err)
	}
}
