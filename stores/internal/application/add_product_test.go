package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestAddProduct_Execute_Success(t *testing.T) {

	// initialize a new module and context
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define a store ID
	storeID := "add-product-store-test-" + time.Now().Format("20060102150405.000000")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Add Product Grocer", "1 Add Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// define an uuid for a product
	id := "add-product-blank-test-" + time.Now().Format("20060102150405.000000")

	// call Execute to add product; expected to succeed.
	product, err := mod.AddProductUC.Execute(ctx, id, storeID, "Whole Milk", "1 gallon", "SKU-MILK-1", 3.50)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if product.Name != "Whole Milk" {
		t.Errorf("expected product name %q, got %q", "Whole Milk", product.Name)
	}
	if product.StoreID != storeID {
		t.Errorf("expected store ID %q, got %q", storeID, product.StoreID)
	}
}

func TestAddProduct_Execute_RejectsBlankName(t *testing.T) {
	// create a new module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define the store id
	storeID := "add-product-blank-store-" + time.Now().Format("20060102150405.000000")

	// create the a new store
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Add Product Blank Grocer", "1 Add Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// define the product id
	id := "add-product-blank-test-" + time.Now().Format("20060102150405.000000")

	// run execute with a blank name; expected to faile.
	_, err = mod.AddProductUC.Execute(ctx, id, storeID, "", "1 gallon", "SKU-MILK-2", 3.50)
	if err == nil {
		t.Fatal("expected Execute to fail on blank name, got nil error")
	}

	// expected the returned error to be ErrProductNameIsBlank
	if !errors.Is(err, domain.ErrProductNameIsBlank) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrProductNameIsBlank, err)
	}
}
