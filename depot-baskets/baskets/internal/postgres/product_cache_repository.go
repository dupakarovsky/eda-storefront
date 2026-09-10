package postgres

import (
	"context"
	"log/slog"
)

// StoreCacheRepository is a type that holds a database connection
// need for accessing the store_cache and product_cache
type ProductCacheRepository struct {
	db     Executor
	logger *slog.Logger
}

// NewCacheRepository implements the NewCacheRepository type
func NewProductCacheRepository(db Executor, logger *slog.Logger) *StoreCacheRepository {
	return &StoreCacheRepository{db: db, logger: logger}
}

func (r *ProductCacheRepository) UpsertProductAdded(ctx context.Context, id, storeID, name, description, sku string, price float64, seq int64) error {

	const insertStmt = `
		INSERT INTO product_cache (id, store_id, name, description, sku, price, last_seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
			SET store_id = EXCLUDED.store_id, 
			name = EXCLUDED.name, 
			description = EXCLUDED.description,
		    sku = EXCLUDED.sku, 
			price = EXCLUDED.price,
			last_seq = EXCLUDED.last_seq
		WHERE product_cache.last_seq < EXCLUDED.last_seq
	`

	exec := executorFrom(ctx, r.db)

	_, err := exec.Exec(ctx, insertStmt, id, storeID, name, description, sku, price, seq)
	return err
}

func (r *ProductCacheRepository) UpdateProductRebranded(ctx context.Context, id, name, description string, seq int64) error {

	const updateStmt = `
		UPDATE product_cache SET name = $2, description = $3, last_seq = $4
		WHERE id = $1 AND last_seq < $4
	`

	exec := executorFrom(ctx, r.db)

	tag, err := exec.Exec(ctx, updateStmt, id, name, description, seq)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		r.logger.Warn("ProductRebranded: no matching product_cache row", "product_id", id, "seq", seq)
	}
	return nil
}

func (r *ProductCacheRepository) ApplyProductPriceDelta(ctx context.Context, id string, delta float64, seq int64) error {

	const updateQuery = `
		UPDATE product_cache SET price = price + $2, last_seq = $3
		WHERE id = $1 AND last_seq < $3
	`
	exec := executorFrom(ctx, r.db)

	tag, err := exec.Exec(ctx, updateQuery, id, delta, seq)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		r.logger.Warn("Product price delta: no matching product_cache row", "product_id", id, "seq", seq)
	}
	return nil
}

func (r *ProductCacheRepository) RemoveProduct(ctx context.Context, id string, seq int64) error {

	const deleteStmt = `DELETE FROM product_cache WHERE id = $1`

	exec := executorFrom(ctx, r.db)

	_, err := exec.Exec(ctx, deleteStmt, id)
	return err
}
