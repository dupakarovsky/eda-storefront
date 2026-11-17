package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/depot/internal/catalog"
	"github.com/jackc/pgx/v5"
)

// CatalogRepository is a any type that has the behavior to execute sql
// statements (pgxpool)
type CatalogRepository struct {
	db Executor
}

// NewCatalogRepository will instantiate a new NewCatalogRepository
func NewCatalogRepository(db Executor) *CatalogRepository {
	return &CatalogRepository{
		db: db,
	}
}

//-----------------------------------------------------------------------------
// CATALOG STORE
//-----------------------------------------------------------------------------

// CreateStore will create a new catalog store from data extracted from the CreateStore Event
// consumed from the stream
func (r *CatalogRepository) CreateStore(ctx context.Context, id, name, location string) error {

	stmt := `
	INSERT INTO catalog_stores (id, name, location, participating)
	VALUES ($1, $2, $3, false)
	ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		location = EXCLUDED.location
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, name, location); err != nil {
		return fmt.Errorf("creating catalog store %q: %w", id, err)
	}

	return nil
}

// RenameStore will perform the renaming of the catalog_store based on data extracted from the
// RenameStore Event consumed to the stream
func (r *CatalogRepository) RenameStore(ctx context.Context, id, name string) error {

	stmt := `
	UPDATE catalog_stores SET name = $2
	WHERE id = $1
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, name); err != nil {
		return fmt.Errorf("renaming catalog store %q: %w", id, err)
	}

	return nil
}

// SetStoreParticipation will update a store's participating property based on the data comming from
// SetStoreParticipation Event consumed from the stream
func (r *CatalogRepository) SetStoreParticipation(ctx context.Context, id string, participating bool) error {

	stmt := `
	UPDATE catalog_stores SET participating = $2
	WHERE id = $1
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, participating); err != nil {
		return fmt.Errorf("setting catalog store participation %q: %w", id, err)
	}

	return nil
}

// FindStoreByID will locate a store in the catalog_store table and return it
func (r *CatalogRepository) FindStoreByID(ctx context.Context, id string) (*catalog.Store, error) {

	stmt := `
	SELECT id, name, location, participating
	FROM catalog_stores
	WHERE id = $1
	`

	exec := executorFrom(ctx, r.db)

	var store catalog.Store

	err := exec.QueryRow(ctx, stmt, id).Scan(&store.ID, &store.Name, &store.Location, &store.Participating)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, catalog.ErrStoreNotFound
		default:
			return nil, fmt.Errorf("finding catalog store %q: %w", id, err)
		}
	}

	return &store, nil
}

//-----------------------------------------------------------------------------
// CATALOG PRODUCT
//-----------------------------------------------------------------------------

// CreateProduct will add a new product to the catalog_products based on the
// CreateProduct event consumed from the stream.
func (r *CatalogRepository) CreateProduct(ctx context.Context, id, storeID, name, description, sku string, price float64, seq uint64) error {

	stmt := `
	INSERT INTO catalog_products (id, store_id, name, description, sku, price, last_applied_seq)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (id) DO UPDATE SET
		store_id = EXCLUDED.store_id,
		name = EXCLUDED.name,
		description = EXCLUDED.description,
		sku = EXCLUDED.sku,
		price = EXCLUDED.price,
		last_applied_seq = EXCLUDED.last_applied_seq
	WHERE catalog_products.last_applied_seq < EXCLUDED.last_applied_seq
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, storeID, name, description, sku, price, seq); err != nil {
		return fmt.Errorf("creating catalog product %q: %w", id, err)
	}

	return nil
}

// depot-baskets/depot/internal/postgres/catalog_repository.go (new)
func (r *CatalogRepository) SetProductPrice(ctx context.Context, id string, price float64, seq uint64) error {

	stmt := `
	UPDATE catalog_products SET price = $2, last_applied_seq = $3
	WHERE id = $1 AND last_applied_seq < $3
	`

	exec := executorFrom(ctx, r.db)

	tag, err := exec.Exec(ctx, stmt, id, price, seq)
	if err != nil {
		return fmt.Errorf("setting catalog product price %q: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return catalog.ErrPriceAdjustmentSkipped
	}

	return nil
}

// RebrandProduct will reband an existing product in the catalog_products table based on the
// data extracted from the RebrandProduct event consumed to the stream
func (r *CatalogRepository) RebrandProduct(ctx context.Context, id, name, description string) error {

	stmt := `
	UPDATE catalog_products SET name = $2, description = $3
	WHERE id = $1
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, name, description); err != nil {
		return fmt.Errorf("rebranding catalog product %q: %w", id, err)
	}

	return nil
}

// AdjustProductPrice will update a price of a product in the catalog_products table
// based on the data extracted from the AdjustProductPrice event consumed from the stream. Whenever there's
// a redelivery of the stream event, we need to make sure the redelivery seq value is above the current
// one that's already in the database for the update to take place. If not, the update is not applied.
func (r *CatalogRepository) AdjustProductPrice(ctx context.Context, id string, delta float64, seq uint64) error {

	stmt := `
	UPDATE catalog_products SET price = price + $2, last_applied_seq = $3
	WHERE id = $1 AND last_applied_seq < $3
	`

	exec := executorFrom(ctx, r.db)

	tag, err := exec.Exec(ctx, stmt, id, delta, seq)
	if err != nil {
		return fmt.Errorf("adjusting catalog product price %q: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return catalog.ErrPriceAdjustmentSkipped
	}

	return nil
}

// DeleteProduct will remove a product from the catalog_products table
// based on the data extracted from the DeleteProduct event consumed from the stream
func (r *CatalogRepository) DeleteProduct(ctx context.Context, id string) error {
	stmt := `
	DELETE FROM catalog_products
	WHERE id = $1
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id); err != nil {
		return fmt.Errorf("deleting catalog product %q: %w", id, err)
	}

	return nil
}

// FindProductByID will locate a product in the catalog_products table and return it
func (r *CatalogRepository) FindProductByID(ctx context.Context, id string) (*catalog.Product, error) {

	stmt := `
	SELECT id, store_id, name, description, sku, price
	FROM catalog_products
	WHERE id = $1
	`
	//
	exec := executorFrom(ctx, r.db)
	//
	var product catalog.Product
	//
	err := exec.QueryRow(ctx, stmt, id).Scan(
		&product.ID, &product.StoreID, &product.Name, &product.Description, &product.SKU, &product.Price,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, catalog.ErrProductNotFound
		default:
			return nil, fmt.Errorf("finding catalog product %q: %w", id, err)
		}
	}
	//
	return &product, nil
}
