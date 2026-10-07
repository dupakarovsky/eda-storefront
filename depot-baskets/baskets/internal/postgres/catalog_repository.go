package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/catalog"
	"github.com/jackc/pgx/v5"
)

var (
	ErrProductNotFound        = errors.New("product not found in catalog")
	ErrPriceAdjustmentSkipped = errors.New("price adjustment skipped: already applied or product not found")
)

type CatalogRepository struct {
	db Executor
}

func NewCatalogRepository(db Executor) *CatalogRepository {
	return &CatalogRepository{
		db: db,
	}
}

//-----------------------------------------------------------------------------
// CATALOG STORE
//-----------------------------------------------------------------------------

// CreateStore will create a new catalog store from data extracted from the stream CreateStore event
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

// RenameStore will perform the renaming of the catalog_store bases on data extracted from the
// stream RenameStore event
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

// SetStoreParticipation will update a store's participating property bases on the data comming from
// SetStoreParticipation event consumed from the stream
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

//-----------------------------------------------------------------------------
// CATALOG PRODUCT
//-----------------------------------------------------------------------------

// CreateProduct will add a new product to the catalog_products based on the
// CreateProduct event published  to the stream.
func (r *CatalogRepository) CreateProduct(ctx context.Context, id, storeID, name, description, sku string, price float64) error {

	stmt := `
	INSERT INTO catalog_products (id, store_id, name, description, sku, price)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (id) DO UPDATE SET
		store_id = EXCLUDED.store_id,
		name = EXCLUDED.name,
		description = EXCLUDED.description,
		sku = EXCLUDED.sku,
		price = EXCLUDED.price
	`

	exec := executorFrom(ctx, r.db)

	if _, err := exec.Exec(ctx, stmt, id, storeID, name, description, sku, price); err != nil {
		return fmt.Errorf("creating catalog product %q: %w", id, err)
	}

	return nil
}

// RebrandProduct will reband an existing product in the catalog_products table based on the
// data extracted from the RebrandProduct event published to the stream
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
// based on the data extracted from the AdjustProductPrice event published to the stream. Whenever there
// a redelivery of the stream event, we need to make sure the redelivery seq value is above the current
// on in the database for the update to take place. If not, the update is not applied.
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
		return ErrPriceAdjustmentSkipped
	}

	return nil
}

// DeleteProduct will remove a product from the catalog_products table
// based on the data extracted from the DeleteProduct event published to the stream
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

	exec := executorFrom(ctx, r.db)

	var product catalog.Product

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

	return &product, nil
}

func (r *CatalogRepository) FindStoreIDsByProductIDs(ctx context.Context, productIDs []string) (map[string]string, error) {

	query := `
		SELECT id, store_id FROM catalog_products
		WHERE id = ANY($1);
	`
	// retrieve the executor
	exec := executorFrom(ctx, r.db)

	// get the rows with results
	rows, err := exec.Query(ctx, query, productIDs)
	if err != nil {
		return nil, fmt.Errorf("querying productID: %v", err)
	}
	defer rows.Close()

	storeIDs := make(map[string]string)

	// begin iteration
	for rows.Next() {

		var productId string
		var storedId string

		// copy record into variables
		if err := rows.Scan(&productId, &storedId); err != nil {
			return nil, fmt.Errorf("iterating over ids: %v", err)
		}

		// record into the map
		storeIDs[productId] = storedId
	}

	// check for iteration errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating item rows: %w", err)
	}

	return storeIDs, nil
}
