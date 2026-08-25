package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	"github.com/jackc/pgx/v5"
)

// ProductRepository defines a type that embeds a concrete implementation of
// the Executor interface
type ProductRepository struct {
	db Executor
}

// NewProductRepository consturcts a new ProductRepository
func NewProductRepository(db Executor) *ProductRepository {
	return &ProductRepository{
		db: db,
	}
}

// Save takes in a Store struct and will persist it into a postgres database
func (r *ProductRepository) Save(ctx context.Context, product *domain.Product) error {
	//  insert query
	insertStmt := `
	INSERT INTO products (id, store_id, name, description, sku, price, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, now())
	ON CONFLICT (id) DO UPDATE SET 
		name = EXCLUDED.name,
		description = EXCLUDED.description,
		sku = EXCLUDED.sku,
		price = EXCLUDED.price,
		updated_at = now()
	`

	// execute the statement passsing the Product's fields to statement
	_, err := r.db.Exec(ctx, insertStmt, product.ID, product.StoreID, product.Name, product.Description, product.SKU, product.Price)
	if err != nil {
		return fmt.Errorf("saving product %q: %W", product.ID, err)
	}

	return nil
}

// FindByID will try to locate a Product record in the database from an ID, returning a reconstituted
// Product if succeeds.
func (r *ProductRepository) FindByID(ctx context.Context, id string) (*domain.Product, error) {

	//  select for upate statement
	selectForUpdateStmt := `
	SELECT id, store_id, name, description, sku, price 
	FROM products WHERE id = $1 
	FOR UPDATE
	`

	// variables that will hold the data from the database
	var (
		productID   string
		storeID     string
		name        string
		description string
		sku         string
		price       float64 // pgx converts NUMERIC to float64 natively
	)

	// execute the query and record the column values into the variables
	err := r.db.QueryRow(ctx, selectForUpdateStmt, id).Scan(&productID, &storeID, &name, &description, &sku, &price)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrProductNotFound
		default:
			return nil, fmt.Errorf("finding product %q: %w", productID, err)
		}
	}

	// reconstitute the values into a Product struct
	product := domain.ReconstituteProduct(productID, storeID, name, description, sku, price)

	return product, nil
}

// Delete will remove a Product record from the database
func (r *ProductRepository) Delete(ctx context.Context, id string) error {

	deleteStmt := `DELETE FROM products WHERE id = $1`

	_, err := r.db.Exec(ctx, deleteStmt, id)
	if err != nil {
		return fmt.Errorf("deleting product %q: %w", id, err)
	}

	return nil
}
