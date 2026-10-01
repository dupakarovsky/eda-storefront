package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dupakarovsky/eda-storefront/ordering/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ShoppingListRepository is a type that has access to the database
// pool of connections and will perform actions against the baskets table
type OrderRepository struct {
	db Executor
}

// NewShoppingListRepository instanitates a new ShoppingListRepository
func NewOrderRepository(db Executor) *OrderRepository {
	return &OrderRepository{
		db: db,
	}
}

// WithLock will start a transaction and run a callback in the transaction is active
func (r *OrderRepository) WithLock(ctx context.Context, fn func(txCtx context.Context) error) error {

	// use the repository pgxpool to Begin a transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beggining transaction: %w", err)
	}

	// store the transaciton object within the context
	txCtx := ctxWithTx(ctx, tx)

	// run the callback within a transaction block
	if err := fn(txCtx); err != nil {
		// we need a new context to rollback the transaction. Otherwise tx.Rollback
		// will try to run on an already expired context and will error immediately
		rbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if rbErr := tx.Rollback(rbCtx); rbErr != nil {
			return fmt.Errorf("rolling back transaction after error %q: %w", err, rbErr)
		}
		return err
	}

	// past this point the callback succeded.
	// ----------------------------------------

	// commit the transaction
	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Save will persist an Order struct into the orders and order_items table
func (r *OrderRepository) Save(ctx context.Context, order *domain.Order) error {

	// get the executor object from the context
	exec := executorFrom(ctx, r.db)

	// insert/update statement into the 'orders' table
	upsertStmt := `
	INSERT INTO orders (id, status, shopping_list_id, updated_at)
	VALUES ($1, $2, $3, now())
	ON CONFLICT (id) DO UPDATE SET
		status = EXCLUDED.status,
		shopping_list_id = EXCLUDED.shopping_list_id,
		updated_at = now()
	`
	// execute the statement first
	if _, err := exec.Exec(ctx, upsertStmt, order.ID, order.Status, order.ShoppingListID); err != nil {
		return fmt.Errorf("saving order %q: %w", order.ID, err)
	}

	// insertion into the 'orders' succeded. now insert into order_items
	// -----------------------------------------------------------------

	if err := r.replaceItems(ctx, exec, order); err != nil {
		return fmt.Errorf("replacing items for order %q: %w", order.ID, err)
	}

	return nil
}

// FindByID will locate an order by it's ID. This won't be run within a transaction
func (r *OrderRepository) FindByID(ctx context.Context, id string) (*domain.Order, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, false)
}

// FindByIDForUpdate will locate an order by it's ID. It'll run within a transaction
func (r *OrderRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.Order, error) {
	return r.find(ctx, executorFrom(ctx, r.db), id, true)
}

// FindByShoppingListID will return an order associated with an shopping list id. This method
// does NOT need to run inside a transaciton block.
func (r *OrderRepository) FindByShoppingListID(ctx context.Context, shoppingListID string) (*domain.Order, error) {

	// select statement for the record
	selectStmt := `
	SELECT id,status
	FROM orders where shopping_list_id = $1;
	`

	// retrieve the transaciton object from the context
	exec := executorFrom(ctx, r.db)

	// variables to received data from the database columns
	var (
		id     string
		status string
	)

	// execute the query
	err := exec.QueryRow(ctx, selectStmt, shoppingListID).Scan(&id, &status)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrOrderNotFound
		default:
			return nil, fmt.Errorf("finding order by shopping list id: %w", err)
		}
	}

	// load this order's items
	items, err := r.loadItems(ctx, exec, id)
	if err != nil {
		return nil, fmt.Errorf("loading order items: %w", err)
	}

	// return the reconstructed domain.Order
	return domain.ReconstituteOrder(id, domain.OrderStatus(status), items, shoppingListID), nil
}

//-------------------------------------------------------------------------------

// replaceItems wiil update the order_items table once the 'order' table has been updated.
// it'll first delete all Items currently associated with an order ID, then insert the ones
// that come from the order.Items
func (r *OrderRepository) replaceItems(ctx context.Context, exec Executor, order *domain.Order) error {

	// first, remove all current items from the order_items associated with this order id
	deleteStmt := `DELETE FROM order_items WHERE order_id = $1`
	if _, err := exec.Exec(ctx, deleteStmt, order.ID); err != nil {
		return fmt.Errorf("clearing items: %w", err)
	}

	// delete successful. now add the new items
	// ----------------------------------------

	// insert new items
	insertStmt := `
	INSERT INTO order_items (order_id, product_id, store_id, quantity)
	VALUES ($1, $2, $3, $4)
	`

	// insert each Item in the Items slice
	for _, item := range order.Items {
		if _, err := exec.Exec(ctx, insertStmt, order.ID, item.ProductID, item.StoreID, item.Quantity); err != nil {
			return fmt.Errorf("inserting item %q: %w", item.ProductID, err)
		}
	}

	// successful insertion
	// ---------------------

	return nil
}

// find is a helper method will return an order in the database with all of it's Items
func (r *OrderRepository) find(ctx context.Context, exec Executor, id string, forUpdate bool) (*domain.Order, error) {

	selectStmt := `
	SELECT id, status, shopping_list_id
	FROM orders
	WHERE id = $1
	`

	if forUpdate {
		selectStmt += `
	FOR UPDATE
	`
	}

	// variables that will take the database values
	var (
		orderID        string
		status         string
		shoppingListID string
	)

	// run the query
	err := exec.QueryRow(ctx, selectStmt, id).Scan(&orderID, &status, &shoppingListID)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrOrderNotFound
		default:
			return nil, fmt.Errorf("finding order %q: %w", id, err)
		}
	}

	// get the items slice from this Order
	items, err := r.loadItems(ctx, exec, orderID)
	if err != nil {
		return nil, fmt.Errorf("loading items for order %q: %w", id, err)
	}

	// recreate the domain.Order type from the values retrieved from the database
	return domain.ReconstituteOrder(orderID, domain.OrderStatus(status), items, shoppingListID), nil
}

// loadItems is halper function that will return an slice of Items by checking all tiem Items associated
// with a particular order
func (r *OrderRepository) loadItems(ctx context.Context, exec Executor, orderID string) ([]domain.Item, error) {

	// select all rows from an order ID
	selectStmt := `
	SELECT product_id, store_id, quantity
	FROM order_items
	WHERE order_id = $1
	`

	// execute the query
	rows, err := exec.Query(ctx, selectStmt, orderID)
	if err != nil {
		return nil, fmt.Errorf("querying items: %w", err)
	}
	defer rows.Close()

	// items will store all returned Items
	var items []domain.Item

	// run the iterator
	for rows.Next() {
		var item domain.Item
		// copy all database values into the Item variable
		if err := rows.Scan(&item.ProductID, &item.StoreID, &item.Quantity); err != nil {
			return nil, fmt.Errorf("scanning item row: %w", err)
		}
		// then append to the slice
		items = append(items, item)
	}

	// verify for interation errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating item rows: %w", err)
	}

	return items, nil
}
