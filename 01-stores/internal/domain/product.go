package domain

import (
	"errors"
	"time"
)

var (
	ErrProductNameIsBlank     = errors.New("the product name cannot be blank")
	ErrProductPriceIsNegative = errors.New("the product price cannot be negative")
	ErrNotAPriceIncrease      = errors.New("the price change would be a decrease")
	ErrNotAPriceDecrease      = errors.New("the price change would be an increase")
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
	if price < 0 {
		return nil, nil, ErrProductPriceIsNegative
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
	// guard against negative values
	if price < 0 {
		return nil, ErrProductPriceIsNegative
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
	}, nil

}

func (p *Product) DecreasePrice(price float64) (Event, error) {

	// guard against negative values
	if price < 0 {
		return nil, ErrProductPriceIsNegative
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
	}, nil
}

// Remove will remove a Product from a Store
func (p *Product) Remove() (Event, error) {
	return ProductRemoved{
		eventBase: eventBase{aggregateID: p.ID, occurredAt: time.Now()},
	}, nil
}
