package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor defines the behaviour for any concrete type to Execute SQL queries
// againts the database.
type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// StoreRepository holds any type the implements the Executor interface
type StoreRepository struct {
	db Executor
}

// NewStoreRepository instantiates a new StoreRepository
func NewStoreRepository(db Executor) *StoreRepository {
	return &StoreRepository{db: db}
}

// Save takes in a Store struct and will persist it into a postgres database
func (r *StoreRepository) Save(ctx context.Context, store *domain.Store) error {

	// insert query
	insertStmt := `
	INSERT INTO stores (id, name, location, participating, updated_at) 
	VALUES ($1, $2, $3, $4, now())
	ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		location = EXCLUDED.location,
		participating = EXCLUDED.participating,
		updated_at = now()
	`
	// execute the statemnt using the store fields to persist into the database
	_, err := r.db.Exec(ctx, insertStmt, store.ID, store.Name, store.Location, store.Participating)
	if err != nil {
		return fmt.Errorf("saving store %q: %w", store.ID, err)
	}

	return nil
}

// FindByID fill try to locate a Store record in the database from an ID, returning a reconstituted
// Store if succeeds.
func (r *StoreRepository) FindByID(ctx context.Context, id string) (*domain.Store, error) {

	// selectStmt for update. Here is inert as it's not called with pgx.Tx
	selectStmt := `
	SELECT id, name, location, participating 
	FROM stores 
	WHERE id = $1
	FOR UPDATE
	`

	// variables that will hold the values from the record
	var (
		storeID       string
		name          string
		location      string
		participating bool
	)

	// execute the QueryRow and store the values into the variables
	err := r.db.QueryRow(ctx, selectStmt, id).Scan(&storeID, &name, &location, &participating)
	if err != nil {
		switch {
		// No records found
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrStoreNotFound
		default:
			return nil, fmt.Errorf("finding store %q: %w", id, err)
		}
	}

	// query successful. Reconstitute the Store struct
	store := domain.ReconstituteStore(storeID, name, location, participating)

	return store, nil
}
