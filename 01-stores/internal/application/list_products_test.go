package application_test

import "testing"

func TestListProducts_Execute_ReturnsAllProductsForStore(t *testing.T) {

	// create a new module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define a id for the store; create the store
	storeID := uniqueID("list-products-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "List Products Grocer", "1 List Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// add a new product
	_, err = mod.AddProductUC.Execute(ctx, uniqueID("list-products-a"), storeID, "Apples", "red delicious", "SKU-LIST-A", 2.50)
	if err != nil {
		t.Fatalf("failed to add first product: %v", err)
	}

	// add another  product
	_, err = mod.AddProductUC.Execute(ctx, uniqueID("list-products-b"), storeID, "Bananas", "yellow, ripe", "SKU-LIST-B", 1.20)
	if err != nil {
		t.Fatalf("failed to add second product: %v", err)
	}

	// total of 2 products should have been added

	// list all products that where added
	products, err := mod.ListProductsUC.Execute(ctx, storeID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// expected to have 2 products added
	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}

	// product 1 expected to be "Apples" and product 2 expected to be "Bananas"
	if products[0].Name != "Apples" || products[1].Name != "Bananas" {
		t.Errorf("expected products ordered [Apples, Bananas], got [%s, %s]", products[0].Name, products[1].Name)
	}
}

func TestListProducts_Execute_ReturnsEmptySliceForStoreWithNoProducts(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	storeID := uniqueID("list-products-empty-store")
	_, err := mod.CreateStoreUC.Execute(ctx, storeID, "Empty Grocer", "1 Empty Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// try to list Products for a store with none; should return an empty slice ("[]")
	products, err := mod.ListProductsUC.Execute(ctx, storeID)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// expected an empty slice, not nil
	if products == nil {
		t.Error("expected an empty, non-nil slice, got nil")
	}

	if len(products) != 0 {
		t.Errorf("expected 0 products, got %d", len(products))
	}
}
