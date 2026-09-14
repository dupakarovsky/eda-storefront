package catalog

// Store represents a store in the system. The information
// from a stores com from subscribing to the stores.store.*
// stream. Any message published is consumed by baskets and
// inserted into the basketsdb database (catalog_stores) table
type Store struct {
	ID            string
	Name          string
	Location      string
	Participating bool
}
