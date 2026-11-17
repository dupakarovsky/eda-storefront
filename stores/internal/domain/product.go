package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrProductNameIsBlank             = errors.New("the product name cannot be blank")
	ErrProductPriceIsNegative         = errors.New("the product price cannot be negative")
	ErrProductPriceHasTooManyDecimals = errors.New("the product price cannot have more than two decimal places") // new
	ErrNotAPriceIncrease              = errors.New("the price change would be a decrease")
	ErrNotAPriceDecrease              = errors.New("the price change would be an increase")
)

// Product represents a product from a Store
type Product struct {
	ID          string
	StoreID     string
	Name        string
	Description string
	SKU         string
	Price       float64
}

// NewProduct constructs a new Product returning also an Event
func NewProduct(id, storeID, name, descrption, sku string, price float64) (*Product, Event, error) {
	// guard against a blank name
	if name == "" {
		return nil, nil, ErrProductNameIsBlank
	}

	// guard against negative values for products
	if err := validatePrice(price); err != nil {
		return nil, nil, err
	}

	product := &Product{
		ID:          id,
		StoreID:     storeID,
		Name:        name,
		Description: descrption,
		SKU:         sku,
		Price:       price,
	}

	// costruct the NewProductAdded event
	event := ProductAdded{
		eventBase:   eventBase{aggregateID: id, occurredAt: time.Now()},
		StoreID:     storeID,
		Name:        name,
		Description: descrption,
		SKU:         sku,
		Price:       price,
	}

	return product, event, nil
}

// Rebrand will update a Product's Name and Description
func (p *Product) Rebrand(name, description string) (Event, error) {
	// guard against blanks
	if name == "" {
		return nil, ErrProductNameIsBlank
	}

	// update the fields
	p.Name = name
	p.Description = description

	// construct a new ProductRebranded event
	event := ProductRebranded{
		eventBase:   eventBase{aggregateID: p.ID, occurredAt: time.Now()},
		Name:        name,
		Description: description,
	}

	return event, nil
}

// IncreasePrice will increase the price of a Product and emit the
// corresponing Event
func (p *Product) IncreasePrice(price float64) (Event, error) {
	// guard against negative values and floaint values with more than 2 decimals
	if err := validatePrice(price); err != nil {
		return nil, err
	}

	// In a price increase new price cannot be smaller than the current price
	if price <= p.Price {
		return nil, ErrNotAPriceIncrease
	}

	// calcluate the price difference
	delta := price - p.Price

	// update the price
	p.Price = price

	// return the ProductPriceIncreased event
	return ProductPriceIncreased{
		eventBase: eventBase{aggregateID: p.ID, occurredAt: time.Now()},
		Delta:     delta,
		Price:     price, // new
	}, nil

}

// DecreasePrice will decresee the price of a Product and emit the
// corresponing Event
func (p *Product) DecreasePrice(price float64) (Event, error) {

	// guard against negative values and floaint values with more than 2 decimals
	if err := validatePrice(price); err != nil {
		return nil, err
	}

	// A price descrease must be smaller than the current price
	if price >= p.Price {
		return nil, ErrNotAPriceDecrease
	}

	// update the price
	delta := price - p.Price
	p.Price = price

	//emite the event
	return ProductPriceDecreased{
		eventBase: eventBase{aggregateID: p.ID, occurredAt: time.Now()},
		Delta:     delta,
		Price:     price, // new
	}, nil
}

// Remove will remove a Product from a Store
func (p *Product) Remove() (Event, error) {
	return ProductRemoved{
		eventBase: eventBase{aggregateID: p.ID, occurredAt: time.Now()},
	}, nil
}

// ReconstituteProduct will return a Product value from the provided parameters.
// Used for retrieving values from the database and creating a new Product instance
func ReconstituteProduct(id, storeID, name, description, sku string, price float64) *Product {
	return &Product{
		ID:          id,
		StoreID:     storeID,
		Name:        name,
		Description: description,
		SKU:         sku,
		Price:       price,
	}
}

//------------------------------------------------------------------------------

// strconv.FormatFloat(price, 'f', -1, 64) sidesteps arithmetic entirely.
// Passing -1 asks for the shortest decimal string that parses back to exactly
// the same float, so a client that sends 1.15 gets "1.15" back, and a client
// that sends 9.995 gets "9.995". That’s the number the client most plausibly
// meant, recovered exactly, and counting the characters after the dot is then
// plain string work. The 'f' format keeps it from switching to exponent notation.
// As a consequence, a value like 0.1 + 0.2 in the noisy test case is rejected:
// the sum is 0.30000000000000004, whose shortest form has 17 decimals,
// which is the right answer for a value that is not a clean cent amount.

func hasAtMostTwoDecimals(price float64) bool {

	s := strconv.FormatFloat(price, 'f', -1, 64)

	// IndexByte returns the index of the first instance of c in s,
	// or -1 if c is not present in s.
	dot := strings.IndexByte(s, '.')

	// 1.2345: len = 6, dot index = 1 => 6-1-1= 4 decimals (fails)
	// 10.234: len = 6, dot index = 2 => 6-2-1= 3 decimals (fails)
	// 123.45: len = 6, dot index = 3 => 6-3-1= 3 decimals (succeeds)
	return dot == -1 || len(s)-dot-1 <= 2
}

func validatePrice(price float64) error {
	if price < 0 {
		return ErrProductPriceIsNegative
	}

	if !hasAtMostTwoDecimals(price) {
		return ErrProductPriceHasTooManyDecimals
	}

	return nil
}
