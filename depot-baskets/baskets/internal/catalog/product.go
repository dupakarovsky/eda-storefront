// baskets/internal/catalog/product.go
package catalog

// Product will capture the information from a product which
// was added from a a store to their inventory. The information is
// collected from Event that was into the jetstream
type Product struct {
	ID          string
	StoreID     string
	Name        string
	Description string
	SKU         string
	Price       float64
}
