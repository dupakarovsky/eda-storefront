package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
	"github.com/jackc/pgx/v5"
)

// BasketRepository is a type that has access to the database
// pool of connections and will perform actions against the baskets table
type BasketRepository struct {
	db Executor
}

// NewBasketRepository will instantiate a new BasketRepository type
func NewBasketRepository(db Executor) *BasketRepository {
	return &BasketRepository{
		db: db,
	}
}

// WithLock will start a transaction
func (r *BasketRepository) WithLock(ctx context.Context, fn func(txCtx context.Context) error) error {

	// initialize the transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// store the transaction object into a new derived context
	txCtx := ctxWithTx(ctx, tx)

	// execute the domain level callback function inside a transaction; rollingback when
	// fail
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(txCtx); rbErr != nil {
			return fmt.Errorf("rolling back transaction after error %q: %w", err, rbErr)
		}
		return err
	}

	// passed here the callback function has succeded; proceed to commit
	// ------------------------------------------------------------------

	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Save will persist data into baskets
func (r *BasketRepository) Save(ctx context.Context, basket *domain.Basket) error {

	exec := executorFrom(ctx, r.db)

	upsertBasketStmt := `
	INSERT INTO baskets (id, status, updated_at)
	VALUES ($1, $2, now())
	ON CONFLICT (id) DO UPDATE SET
		status = EXCLUDED.status,
		updated_at = now()
	`

	if _, err := exec.Exec(ctx, upsertBasketStmt, basket.ID, string(basket.Status)); err != nil {
		return fmt.Errorf("saving basket %q: %w", basket.ID, err)
	}

	if err := r.replaceItems(ctx, exec, basket); err != nil {
		return fmt.Errorf("replacing items for basket %q: %w", basket.ID, err)
	}

	return nil
}

// replaceItems will remove all previous entries in the basket_items table that are associates with
// a basket_id
func (r *BasketRepository) replaceItems(ctx context.Context, exec Executor, basket *domain.Basket) error {

	// remove all items from the basket_id
	deleteStmt := `
	DELETE FROM basket_items
	WHERE basket_id = $1
	`

	if _, err := exec.Exec(ctx, deleteStmt, basket.ID); err != nil {
		return fmt.Errorf("clearing items: %w", err)
	}

	// then insert new ones
	insertStmt := `
	INSERT INTO basket_items (basket_id, product_id, quantity)
	VALUES ($1, $2, $3)
	`

	for _, item := range basket.Items {
		if _, err := exec.Exec(ctx, insertStmt, basket.ID, item.ProductID, item.Quantity); err != nil {
			return fmt.Errorf("inserting item %q: %w", item.ProductID, err)
		}
	}

	return nil
}

// =============================================================================

func (r *BasketRepository) FindByID(ctx context.Context, id string) (*domain.Basket, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, false)
}

// FindByIDForUpdate will locate an Basket by it's ID and will lock it that particular basket
// inside a transaction.
func (r *BasketRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.Basket, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, true)
}

func (r *BasketRepository) find(ctx context.Context, exec Executor, id string, forUpdate bool) (*domain.Basket, error) {

	selectStmt := `
	SELECT id, status
	FROM baskets
	WHERE id = $1
	`
	if forUpdate {
		selectStmt += `
	FOR UPDATE
	`
	}

	var (
		basketID string
		status   string
	)

	err := exec.QueryRow(ctx, selectStmt, id).Scan(&basketID, &status)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrBasketNotFound
		default:
			return nil, fmt.Errorf("finding basket %q: %w", id, err)
		}
	}

	// retrieve all items that are in this basket
	items, err := r.loadItems(ctx, exec, basketID)
	if err != nil {
		return nil, fmt.Errorf("loading items for basket %q: %w", id, err)
	}

	// recreate the Basket domain from the database records
	return domain.ReconstituteBasket(basketID, domain.BasketStatus(status), items), nil
}

// loadItems will return all the Items in a particular basket from the basket_items table
func (r *BasketRepository) loadItems(ctx context.Context, exec Executor, basketID string) ([]domain.Item, error) {

	selectStmt := `
	SELECT product_id, quantity
	FROM basket_items
	WHERE basket_id = $1
	`

	// execute teh query
	rows, err := exec.Query(ctx, selectStmt, basketID)
	if err != nil {
		return nil, fmt.Errorf("querying items: %w", err)
	}
	defer rows.Close()

	// items will hold a all the different items
	var items []domain.Item

	// start the iterator
	for rows.Next() {
		var item domain.Item
		// copy all record into the item fields
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, fmt.Errorf("scanning item row: %w", err)
		}
		// append current item to the slice
		items = append(items, item)
	}

	// very for iteration errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating item rows: %w", err)
	}

	return items, nil
}
