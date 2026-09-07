package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestRemoveProduct_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("remove-product-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Remove Product Grocer", "1 Remove Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	productID := uniqueID("remove-product-test")
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Butter", "salted", "SKU-REM-1", 4.50)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	// remove the product; expected to succeed
	if err := mod.RemoveProductUC.Execute(ctx, productID); err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// try to locate the memoved product; expected to return a ErrProductNotFound
	_, err = mod.ProductRepo.FindByID(ctx, productID)
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("expected product to be gone after removal, FindByID returned: %v", err)
	}
}

func TestRemoveProduct_Execute_RejectsUnknownProduct(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// try to remove an non-existing product; expected to fail
	err := mod.RemoveProductUC.Execute(ctx, uniqueID("remove-product-unknown"))
	if err == nil {
		t.Fatal("expected Execute to fail on an unknown product, got nil error")
	}

	// expected the error to be ErrProductNotFound
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrProductNotFound, err)
	}
}
