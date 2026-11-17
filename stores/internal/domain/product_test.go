package domain

import (
	"errors"
	"testing"
)

// newTestProduct is a helper function that instantiates a new Product
func newTestProduct(t *testing.T) *Product {
	t.Helper()

	product, _, err := NewProduct("product-1", "store-1", "Pepperoni Pizza", "12-inch, classic", "SKU-001", 9.99)
	if err != nil {
		t.Fatalf("failed to create test product: %v", err)
	}

	return product
}

func TestNewProduct(t *testing.T) {

	// 1st Test: Success
	t.Run("valid input creates a product and emits ProductAdded", func(t *testing.T) {

		// create a new product
		product, event, err := NewProduct("product-1", "store-1", "Pepperoni Pizza", "12-inch, classic", "SKU-001", 9.99)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// expected  field to match
		if product.ID != "product-1" {
			t.Fatalf("product ID not set as expected: %+v", product)
		}

		// expected  field to match
		if product.StoreID != "store-1" {
			t.Fatalf("product StoreID not set as expected: %+v", product)

		}

		// expected all fields to match
		if product.Name != "Pepperoni Pizza" {
			t.Fatalf("product Name not set as expected: %+v", product)

		}
		// expected all fields to match
		if product.Description != "12-inch, classic" {
			t.Fatalf("product Description not set as expected: %+v", product)

		}

		// expected all fields to match
		if product.SKU != "SKU-001" {
			t.Fatalf("product SKU not set as expected: %+v", product)

		}

		// expected all fields to match
		if product.Price != 9.99 {
			t.Fatalf("product Price not set as expected: %+v", product)
		}

		// expect the event type to be ProductAdded
		added, ok := event.(ProductAdded)
		if !ok {
			t.Fatalf("expected ProductAdded event, got %T", event)
		}

		if added.AggregateID() != "product-1" {
			t.Errorf("expected aggregate id product-1, got %s", added.AggregateID())
		}

		if added.EventName() != ProductAddedEvent {
			t.Errorf("expected event name %s, got %s", ProductAddedEvent, added.EventName())
		}

		if added.StoreID != "store-1" {
			t.Errorf("event payload mismatch in StoreID: %+v", added.StoreID)
		}

		if added.Name != "Pepperoni Pizza" {
			t.Errorf("event payload mismatch in Name: %+v", added.Name)
		}

		if added.Price != 9.99 {
			t.Errorf("event payload mismatch in Price: %+v", added.Price)
		}
	})

	// 2nd Test: Failure
	t.Run("blank name returns error", func(t *testing.T) {

		// create a product with a blank Name; expect to fail
		product, event, err := NewProduct("product-1", "store-1", "", "desc", "SKU-001", 9.99)
		if !errors.Is(err, ErrProductNameIsBlank) {
			t.Fatalf("expected ErrProductNameIsBlank, got %v", err)
		}
		// expected no product or event created.
		if product != nil || event != nil {
			t.Errorf("expected nil product and event on error")
		}
	})

	// 3rd Test: Failure
	t.Run("negative price returns error", func(t *testing.T) {
		// create a Product with negative Price; expected to fail
		_, _, err := NewProduct("product-1", "store-1", "Pepperoni Pizza", "desc", "SKU-001", -1.00)
		if !errors.Is(err, ErrProductPriceIsNegative) {
			t.Fatalf("expected ErrProductPriceIsNegative, got %v", err)
		}
	})
}

func TestProduct_Rebrand(t *testing.T) {
	// 1st Test: Success
	t.Run("valid name rebrands and emits ProductRebranded", func(t *testing.T) {

		// create a new Product
		product := newTestProduct(t)

		// invoke Rebrand; expected to succeed.
		event, err := product.Rebrand("Veggie Supreme", "12-inch, loaded")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		// properties should match
		if product.Name != "Veggie Supreme" || product.Description != "12-inch, loaded" {
			t.Errorf("expected product fields updated, got %+v", product)
		}

		// event type expected to be ProductRebranded
		rebranded, ok := event.(ProductRebranded)
		if !ok {
			t.Fatalf("expected ProductRebranded, got %T", event)
		}
		// event expected to hold Product's Name and Description
		if rebranded.Name != "Veggie Supreme" || rebranded.Description != "12-inch, loaded" {
			t.Errorf("event payload mismatch: %+v", rebranded)
		}
	})

	// 2nd Test: Failure
	t.Run("blank name returns error and does not mutate product", func(t *testing.T) {
		// create a new product
		product := newTestProduct(t)
		// store the original Name property for comparisson.
		originalName := product.Name

		// call Rebrand with blank Name; expected to fail
		event, err := product.Rebrand("", "desc")
		if !errors.Is(err, ErrProductNameIsBlank) {
			t.Fatalf("expected ErrProductNameIsBlank, got %v", err)
		}
		// expected no event emited and Name to be kept the same
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if product.Name != originalName {
			t.Errorf("expected product name unchanged, got %s", product.Name)
		}
	})
}

func TestProduct_IncreasePrice(t *testing.T) {

	// 1st Test: Success
	t.Run("higher price succeeds and emits ProductPriceIncreased", func(t *testing.T) {

		// create a new Product
		product := newTestProduct(t)

		// invoke the IncreasePrice method; expected to succeed
		event, err := product.IncreasePrice(12.99)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if product.Price != 12.99 {
			t.Errorf("expected price 12.99, got %v", product.Price)
		}

		// expected the event type to be ProductPriceIncreased
		increased, ok := event.(ProductPriceIncreased)
		if !ok {
			t.Fatalf("expected ProductPriceIncreased, got %T", event)
		}

		// delta expected be +3
		if increased.Delta != 3.00 {
			t.Errorf("expected delta 3.00, got %v", increased.Delta)
		}
	})

	// 2nd Test: Failure
	t.Run("equal price fails", func(t *testing.T) {

		// create a new Product
		product := newTestProduct(t)
		originalPrice := product.Price

		// invoke IncreasePrice with the same price; expected to failt
		event, err := product.IncreasePrice(originalPrice)
		if !errors.Is(err, ErrNotAPriceIncrease) {
			t.Fatalf("expected ErrNotAPriceIncrease, got %v", err)
		}
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		if product.Price != originalPrice {
			t.Errorf("expected price unchanged, got %v", product.Price)
		}
	})

	// 3rd Test: Failure
	t.Run("lower price fails", func(t *testing.T) {
		product := newTestProduct(t)

		// invoke IncreasePrice with a lower value than current price;
		// expected to fail
		_, err := product.IncreasePrice(5.00)
		if !errors.Is(err, ErrNotAPriceIncrease) {
			t.Fatalf("expected ErrNotAPriceIncrease, got %v", err)
		}
	})

	// 4th Test: Failure
	t.Run("negative price fails", func(t *testing.T) {
		product := newTestProduct(t)

		_, err := product.IncreasePrice(-5.00)
		if !errors.Is(err, ErrProductPriceIsNegative) {
			t.Fatalf("expected ErrProductPriceIsNegative, got %v", err)
		}
	})
}

func TestProduct_DecreasePrice(t *testing.T) {

	// 1st Test: Success
	t.Run("lower price succeeds and emits ProductPriceDecreased", func(t *testing.T) {

		// create a new Product
		product := newTestProduct(t)

		// invoke DecreasePrice; expected to succeed
		event, err := product.DecreasePrice(7.99)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		//
		if product.Price != 7.99 {
			t.Errorf("expected price 7.99, got %v", product.Price)
		}

		// event type expected to be ProductPriceDecreased
		decreased, ok := event.(ProductPriceDecreased)
		if !ok {
			t.Fatalf("expected ProductPriceDecreased, got %T", event)
		}

		// expected delta
		if decreased.Delta != -2.00 {
			t.Errorf("expected delta -2.00, got %v", decreased.Delta)
		}
	})

	// 2nd Test: Failure
	t.Run("equal price fails", func(t *testing.T) {
		// create a new Product
		product := newTestProduct(t)
		originalPrice := product.Price

		// invoke DecreasePrice with the same Price; expected to fail
		event, err := product.DecreasePrice(originalPrice)
		if !errors.Is(err, ErrNotAPriceDecrease) {
			t.Fatalf("expected ErrNotAPriceDecrease, got %v", err)
		}
		// expected no event emited
		if event != nil {
			t.Errorf("expected nil event on error, got %+v", event)
		}
		// expected original price to remain
		if product.Price != originalPrice {
			t.Errorf("expected price unchanged, got %v", product.Price)
		}
	})

	// 3rd Test: Failure
	t.Run("higher price fails", func(t *testing.T) {
		// create a new Product
		product := newTestProduct(t)

		// invoke DecreasePricw with a higher Price; expected to fail
		_, err := product.DecreasePrice(15.00)
		if !errors.Is(err, ErrNotAPriceDecrease) {
			t.Fatalf("expected ErrNotAPriceDecrease, got %v", err)
		}
	})

	// 4th Test: Failure
	t.Run("negative price fails", func(t *testing.T) {
		// create a new Product
		product := newTestProduct(t)

		// invoke DecreasePricw with a negative value; expected to fail
		_, err := product.DecreasePrice(-5.00)
		if !errors.Is(err, ErrProductPriceIsNegative) {
			t.Fatalf("expected ErrProductPriceIsNegative, got %v", err)
		}
	})
}

func TestProduct_PriceDecimals(t *testing.T) {
	noisy := 0.1
	noisy += 0.2

	valid := []struct {
		name  string
		price float64
	}{
		{"zero", 0},
		{"whole number", 5},
		{"one decimal", 9.5},
		{"two decimals", 9.99},
		{"naive check trips on 1.15", 1.15},
		{"naive check trips on 4.35", 4.35},
		{"naive check trips on 8.2", 8.2},
		{"largest NUMERIC(10,2) value", 99999999.99},
	}

	for _, tc := range valid {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			if _, _, err := NewProduct("product-1", "store-1", "Widget", "desc", "SKU-1", tc.price); err != nil {
				t.Fatalf("expected %v to be accepted, got error: %v", tc.price, err)
			}
		})
	}

	invalid := []struct {
		name  string
		price float64
	}{
		{"three decimals", 9.995},
		{"tenth of a cent", 0.001},
		{"1.005", 1.005},
		{"float noise from arithmetic", noisy},
	}

	for _, tc := range invalid {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			product, event, err := NewProduct("product-1", "store-1", "Widget", "desc", "SKU-1", tc.price)
			if !errors.Is(err, ErrProductPriceHasTooManyDecimals) {
				t.Fatalf("expected ErrProductPriceHasTooManyDecimals for %v, got %v", tc.price, err)
			}
			if product != nil || event != nil {
				t.Errorf("expected nil product and event on error")
			}
		})
	}
}

func TestProduct_NegativePriceWithDecimals_ReportsNegativeFirst(t *testing.T) {
	_, _, err := NewProduct("product-1", "store-1", "Widget", "desc", "SKU-1", -1.005)
	if !errors.Is(err, ErrProductPriceIsNegative) {
		t.Fatalf("expected ErrProductPriceIsNegative, got %v", err)
	}
}

func TestProduct_IncreasePrice_RejectsTooManyDecimals(t *testing.T) {
	product := newTestProduct(t)
	original := product.Price

	event, err := product.IncreasePrice(12.995)
	if !errors.Is(err, ErrProductPriceHasTooManyDecimals) {
		t.Fatalf("expected ErrProductPriceHasTooManyDecimals, got %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event on error, got %+v", event)
	}
	if product.Price != original {
		t.Errorf("expected price unchanged at %v, got %v", original, product.Price)
	}
}

func TestProduct_DecreasePrice_RejectsTooManyDecimals(t *testing.T) {
	product := newTestProduct(t)
	original := product.Price

	event, err := product.DecreasePrice(7.001)
	if !errors.Is(err, ErrProductPriceHasTooManyDecimals) {
		t.Fatalf("expected ErrProductPriceHasTooManyDecimals, got %v", err)
	}
	if event != nil {
		t.Errorf("expected nil event on error, got %+v", event)
	}
	if product.Price != original {
		t.Errorf("expected price unchanged at %v, got %v", original, product.Price)
	}
}

func TestProduct_PriceEvents_CarryAbsolutePrice(t *testing.T) {
	t.Run("increase", func(t *testing.T) {
		product := newTestProduct(t)

		event, err := product.IncreasePrice(12.99)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		increased, ok := event.(ProductPriceIncreased)
		if !ok {
			t.Fatalf("expected ProductPriceIncreased, got %T", event)
		}
		if increased.Price != 12.99 {
			t.Errorf("expected event price 12.99, got %v", increased.Price)
		}
	})

	t.Run("decrease", func(t *testing.T) {
		product := newTestProduct(t)

		event, err := product.DecreasePrice(7.99)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		decreased, ok := event.(ProductPriceDecreased)
		if !ok {
			t.Fatalf("expected ProductPriceDecreased, got %T", event)
		}
		if decreased.Price != 7.99 {
			t.Errorf("expected event price 7.99, got %v", decreased.Price)
		}
	})
}

func TestProduct_Remove(t *testing.T) {
	// create a new Product
	product := newTestProduct(t)

	// call the Remove method; expected to succeed
	event, err := product.Remove()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// expected the event type to be ProductRemoved
	removed, ok := event.(ProductRemoved)
	if !ok {
		t.Fatalf("expected ProductRemoved, got %T", event)
	}
	// expected to store the AggregateID
	if removed.AggregateID() != "product-1" {
		t.Errorf("expected aggregate id product-1, got %s", removed.AggregateID())
	}
}
