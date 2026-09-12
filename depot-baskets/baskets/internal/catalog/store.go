package catalog

// Store represents a new store added to the system, which
// information was extracted from the StoreCreated Event and
// published to the stream.
type Store struct {
	ID            string
	Name          string
	Location      string
	Participating bool
}
