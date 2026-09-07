package application_test

import (
	"errors"
	"testing"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestRebrandProduct_Execute_Succeeds(t *testing.T) {

	// create a new module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define a id for the store
	storeID := uniqueID("rebrand-product-store")

	// create the store
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Rebrand Product Grocer", "1 Rebrand Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// define a id for the product
	productID := uniqueID("rebrand-product-test")

	// create a new product
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Original Product", "original desc", "SKU-REB-1", 5.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	// execute the rebranding; expected to succeed
	product, err := mod.RebrandProductUC.Execute(ctx, productID, "New Product Name", "new desc")
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// expected the rebranded product to match our new name
	if product.Name != "New Product Name" {
		t.Errorf("expected product name %q, got %q", "New Product Name", product.Name)
	}

	if product.Description != "new desc" {
		t.Errorf("expected description %q, got %q", "new desc", product.Description)
	}
}

func TestRebrandProduct_Execute_RejectsBlankName(t *testing.T) {
	// create a new modue
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define an ID for the store
	storeID := uniqueID("rebrand-product-blank-store")
	// create the store
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Rebrand Product Blank Grocer", "1 Rebrand Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// create an id for the product
	productID := uniqueID("rebrand-product-blank-test")

	// create a new product
	_, err = mod.AddProductUC.Execute(ctx, productID, storeID, "Original Product", "original desc", "SKU-REB-2", 5.00)
	if err != nil {
		t.Fatalf("failed to add product: %v", err)
	}

	// try re brand the product with a blank name; expected to fail
	_, err = mod.RebrandProductUC.Execute(ctx, productID, "", "new desc")
	if err == nil {
		t.Fatal("expected Execute to fail on blank name, got nil error")
	}

	// expected the error to be ErrProductNameIsBlank
	if !errors.Is(err, domain.ErrProductNameIsBlank) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrProductNameIsBlank, err)
	}
}
