package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/depot/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ShoppingListRepository is a type that has access to the database
// pool of connections and will perform actions against the baskets table
type ShoppingListRepository struct {
	db Executor
}

// NewShoppingListRepository instanitates a new ShoppingListRepository
func NewShoppingListRepository(db Executor) *ShoppingListRepository {
	return &ShoppingListRepository{
		db: db,
	}
}

// WithLock will start a transaction and run a callback in the transaction is active
func (r *ShoppingListRepository) WithLock(ctx context.Context, fn func(txCtx context.Context) error) error {

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

// Save will persist a Shopping List to the local database under shoppin_list table
func (r *ShoppingListRepository) Save(ctx context.Context, shoppingList *domain.ShoppingList) error {

	exec := executorFrom(ctx, r.db)

	// update the 'initiated_at' and "completed_at" fields depending on the value of status
	// using the COALESCE function to update with the non NULL value
	upsertStmt := `
	INSERT INTO shopping_lists (id, status, updated_at, initiated_at, completed_at)
	VALUES ($1, $2, now(),
		CASE WHEN $2 = 'active' THEN now() ELSE NULL END,
		CASE WHEN $2 = 'completed' THEN now() ELSE NULL END)
	ON CONFLICT (id) DO UPDATE SET
		status = EXCLUDED.status,
		updated_at = now(),
		initiated_at = COALESCE(shopping_lists.initiated_at, EXCLUDED.initiated_at),
		completed_at = COALESCE(shopping_lists.completed_at, EXCLUDED.completed_at)
	`

	// execute the command
	if _, err := exec.Exec(ctx, upsertStmt, shoppingList.ID, string(shoppingList.Status)); err != nil {
		return fmt.Errorf("saving shopping list %q: %w", shoppingList.ID, err)
	}

	// when the update happens, all Items from the "shopping_list_stop" table will be deleted
	// and inserted again with the new entries from

	//
	if err := r.replaceStops(ctx, exec, shoppingList); err != nil {
		return fmt.Errorf("replacing stops for shopping list %q: %w", shoppingList.ID, err)
	}

	return nil
}

// replaceStops will remove all Items from each of Stops of the current list that are referenced in the
// database, then remove all of the Stops in this list. Then it'll proceed to insert the Stops one by one.
// At every Stop added, it'll insert the all of Items from that Stop, then proceed to the next.
func (r *ShoppingListRepository) replaceStops(ctx context.Context, exec Executor, shoppingList *domain.ShoppingList) error {

	// statement to remove all Items from the all the stops this shopping list
	deleteItemsStmt := `DELETE FROM shopping_list_items WHERE shopping_list_id = $1`
	if _, err := exec.Exec(ctx, deleteItemsStmt, shoppingList.ID); err != nil {
		return fmt.Errorf("clearing items: %w", err)
	}

	// past here no, all of the Stops from this list have no items
	// ------------------------------------------------------------

	// statement to remove all Stops from a particular "shopping_list"
	deleteStopsStmt := `DELETE FROM shopping_list_stops WHERE shopping_list_id = $1`
	if _, err := exec.Exec(ctx, deleteStopsStmt, shoppingList.ID); err != nil {
		return fmt.Errorf("clearing stops: %w", err)
	}

	// past here this shopping lists has no Stops
	// ------------------------------------------

	// statement to insert Stops back into this shopping list
	insertStopStmt := `
	INSERT INTO shopping_list_stops (shopping_list_id, store_id, store_name, store_location)
	VALUES ($1, $2, $3, $4)
	`

	// statement to insert Items back into the Stops for this shopping list
	insertItemStmt := `
	INSERT INTO shopping_list_items (shopping_list_id, store_id, product_id, quantity)
	VALUES ($1, $2, $3, $4)
	`

	for storeID, stop := range shoppingList.Stops {

		// First insert each Stop back into the list
		if _, err := exec.Exec(ctx, insertStopStmt, shoppingList.ID, storeID, stop.StoreName, stop.StoreLocation); err != nil {
			return fmt.Errorf("inserting stop %q: %w", storeID, err)
		}

		// then insert the each Item into the current Stop
		for _, item := range stop.Items {
			if _, err := exec.Exec(ctx, insertItemStmt, shoppingList.ID, storeID, item.ProductID, item.Quantity); err != nil {
				return fmt.Errorf("inserting item %q: %w", item.ProductID, err)
			}
		}
	}

	return nil
}

func (r *ShoppingListRepository) FindByID(ctx context.Context, id string) (*domain.ShoppingList, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, false)
}

func (r *ShoppingListRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.ShoppingList, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, true)
}

// find is a helper method that will return a shopping list
func (r *ShoppingListRepository) find(ctx context.Context, exec Executor, id string, forUpdate bool) (*domain.ShoppingList, error) {

	selectStmt := `
	SELECT id, status
	FROM shopping_lists
	WHERE id = $1
	`
	if forUpdate {
		selectStmt += `
	FOR UPDATE
	`
	}

	var (
		listID string
		status string
	)

	// retrieve the shopping list data (id and status) from the shopping_list table
	err := exec.QueryRow(ctx, selectStmt, id).Scan(&listID, &status)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrShoppingListNotFound
		default:
			return nil, fmt.Errorf("finding shopping list %q: %w", id, err)
		}
	}

	// then retrieve all the Stops associated to this list. It's a map of Stops keyed by a "store_id"
	stops, err := r.loadStops(ctx, exec, listID)
	if err != nil {
		return nil, fmt.Errorf("loading stops for shopping list %q: %w", id, err)
	}

	// then reconstruct the domain.ShoppingList
	return domain.ReconstituteShoppingList(listID, domain.ShoppingListStatus(status), stops), nil
}

// loadStops will create a map of Stops keyed by a store id, which is the Stops data strucure
// need by the shopping list
func (r *ShoppingListRepository) loadStops(ctx context.Context, exec Executor, listID string) (map[string]*domain.Stop, error) {

	// retrieve all Stops from a list
	selectStopsStmt := `
	SELECT store_id, store_name, store_location
	FROM shopping_list_stops
	WHERE shopping_list_id = $1
	`

	rows, err := exec.Query(ctx, selectStopsStmt, listID)
	if err != nil {
		return nil, fmt.Errorf("querying stops: %w", err)
	}
	defer rows.Close()

	// stops will hold each Stop returned by the query
	stops := make(map[string]*domain.Stop)

	for rows.Next() {
		var storeID, storeName, storeLocation string
		if err := rows.Scan(&storeID, &storeName, &storeLocation); err != nil {
			return nil, fmt.Errorf("scanning stop row: %w", err)
		}
		// associate each Stop to and store id
		stops[storeID] = &domain.Stop{
			StoreName:     storeName,
			StoreLocation: storeLocation,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stop rows: %w", err)
	}

	// all Stops where copied to the stops map
	// now add Items to each Stop
	// ---------------------------------------

	selectItemsStmt := `
	SELECT store_id, product_id, quantity
	FROM shopping_list_items
	WHERE shopping_list_id = $1
	`

	itemRows, err := exec.Query(ctx, selectItemsStmt, listID)
	if err != nil {
		return nil, fmt.Errorf("querying items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var storeID, productID string
		var quantity int
		if err := itemRows.Scan(&storeID, &productID, &quantity); err != nil {
			return nil, fmt.Errorf("scanning item row: %w", err)
		}

		// add in new Item if the Stop already exists in the map
		if stop, exists := stops[storeID]; exists {
			stop.Items = append(stop.Items, domain.Item{ProductID: productID, Quantity: quantity})
		}
	}

	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating item rows: %w", err)
	}

	return stops, nil
}
