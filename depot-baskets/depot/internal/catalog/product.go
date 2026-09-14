package catalog

// Product represents a product added by a stores. The information
// from a product comes by subscribing to the  stores.product.*
// stream. Any message published is consumed by baskets and
// inserted into the basketsdb database (catalog_products) table
type Product struct {
	ID          string
	StoreID     string
	Name        string
	Description string
	SKU         string
	Price       float64
}
