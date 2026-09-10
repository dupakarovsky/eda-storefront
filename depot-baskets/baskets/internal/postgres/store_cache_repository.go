// baskets/internal/postgres/cache_repository.go
package postgres

import (
	"context"
	"log/slog"
)

// StoreCacheRepository is a type that holds a database connection
// need for accessing the store_cache and product_cache
type StoreCacheRepository struct {
	db     Executor
	logger *slog.Logger
}

// NewCacheRepository implements the NewCacheRepository type
func NewCacheRepository(db Executor, logger *slog.Logger) *StoreCacheRepository {
	return &StoreCacheRepository{db: db, logger: logger}
}

// UpsertStoreCreated will try to insert a new record into the store_cache table
// using data from a StoreCreated stream event
func (r *StoreCacheRepository) UpsertStoreCreated(ctx context.Context, id, name, location string, seq int64) error {

	// insert statement
	const insertStmt = `
		INSERT INTO store_cache (id, name, location, participating, last_seq)
		VALUES ($1, $2, $3, false, $4)
		ON CONFLICT (id) DO UPDATE
			SET name = EXCLUDED.name, 
			location = EXCLUDED.location, 
			last_seq = EXCLUDED.last_seq
		WHERE store_cache.last_seq < EXCLUDED.last_seq
	`
	// verify if the context has a transaction object inside; if it does
	// used it instead of the plain db
	exec := executorFrom(ctx, r.db)

	_, err := exec.Exec(ctx, insertStmt, id, name, location, seq)
	return err
}

// UpdateStoreRebranded will perform an an update in a store_cache table
// using data from a StoreRebranded stream event
func (r *StoreCacheRepository) UpdateStoreRebranded(ctx context.Context, id, name string, seq int64) error {

	// update statement to run
	const updateStmt = `
		UPDATE store_cache SET name = $2, last_seq = $3
		WHERE id = $1 AND last_seq < $3
	`

	// verify if the context has a transaction object inside; if it does
	// used it instead of the plain db
	exec := executorFrom(ctx, r.db)

	// execute a the command check if succeded.
	tag, err := exec.Exec(ctx, updateStmt, id, name, seq)
	if err != nil {
		return err
	}

	// veirfy if there where any rows affected
	if tag.RowsAffected() == 0 {
		r.logger.Warn("StoreRebranded: no matching store_cache row", "store_id", id, "seq", seq)
	}
	return nil
}

// UpdateStoreParticipation will perform an update on the store_cache table by
// passing the data from a StoreParticipation event
func (r *StoreCacheRepository) UpdateStoreParticipation(ctx context.Context, id string, participating bool, seq int64) error {

	const updateStmt = `
		UPDATE store_cache SET participating = $2, last_seq = $3
		WHERE id = $1 AND last_seq < $3
	`

	// verify if the context has a transaction object inside; if it does
	// used it instead of the plain db
	exec := executorFrom(ctx, r.db)

	// execute the command
	tag, err := exec.Exec(ctx, updateStmt, id, participating, seq)
	if err != nil {
		return err
	}

	// check if there where any rows affected by the command
	if tag.RowsAffected() == 0 {
		r.logger.Warn("StoreParticipation update: no matching store_cache row", "store_id", id, "seq", seq)
	}
	return nil
}
