package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestIncreaseProductPrice_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("increase-price-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Increase Price Grocer", "1 Increase Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	productID := uniqueID("increase-price-test")
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Bread", "sourdough loaf", "SKU-INC-1", 4.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	// increse product price from 4 to 5; expected to succedd
	product, err := mod.IncreaseProductPriceUC.Execute(ctx, productID, 5.00)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if product.Price != 5.00 {
		t.Errorf("expected price %v, got %v", 5.00, product.Price)
	}
}

func TestIncreaseProductPrice_Execute_RejectsNonIncrease(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("increase-price-reject-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Increase Price Reject Grocer", "1 Increase Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	productID := uniqueID("increase-price-reject-test")
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Bread", "sourdough loaf", "SKU-INC-2", 4.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	// try to call IncreaseProductPrice from 4 to 4; expected to fail
	_, err = mod.IncreaseProductPriceUC.Execute(ctx, productID, 4.00)
	if err == nil {
		t.Fatal("expected Execute to fail on an equal target price, got nil error")
	}

	// expected error type to be ErrNotAPriceIncrease
	if !errors.Is(err, domain.ErrNotAPriceIncrease) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrNotAPriceIncrease, err)
	}
}
