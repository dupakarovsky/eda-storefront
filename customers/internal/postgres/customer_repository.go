package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	"github.com/jackc/pgx/v5"
)

// CustomerRepository defines a concrete type that can communicate
// with the customersdb database by implementing an Executor
type CustomerRepository struct {
	db Executor
}

// NewCusomerRepository creates a new CusomerRepository
func NewCusomerRepository(db Executor) *CustomerRepository {
	return &CustomerRepository{
		db: db,
	}
}

// WithLock will start a transaction and run a callback in the transaction is active
func (r *CustomerRepository) WithLock(ctx context.Context, fn func(txCtx context.Context) error) error {

	// start a transaction block
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// create the enriched context with the tx object
	txCtx := ctxWithTx(ctx, tx)

	// execute the callback within the tranaction block
	if err := fn(txCtx); err != nil {
		// create a new context with a timeout to rollback the transaction
		rbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if rbErr := tx.Rollback(rbCtx); rbErr != nil {
			return fmt.Errorf("rolling back transaction after error %q: %w", rbErr, rbErr)
		}
		return err
	}

	// passed here the transaction succeeded
	// --------------------------------------

	// commit
	if err := tx.Commit(txCtx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Save will persist the Customer into the database
func (r *CustomerRepository) Save(ctx context.Context, customer *domain.Customer) error {

	// query to insert into the database
	updateQuery := `
	INSERT INTO customers (id, name, sms_number, enabled, updated_at)
	VALUES ($1, $2, $3, $4, now())
	ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		sms_number = EXCLUDED.sms_number,
		enabled = EXCLUDED.enabled,
		updated_at = now()

	`

	// get the transaction object from the database
	exec := executorFrom(ctx, r.db)

	// execute the query to record
	_, err := exec.Exec(ctx, updateQuery, customer.ID, customer.Name, customer.SmsNumber, customer.Enabled)
	if err != nil {
		return fmt.Errorf("saving customer %q: %w", customer.ID, err)
	}

	return nil
}

// FindByID will locate a customer in the database using the provided ID and return
// a domain reconstituted type. Does NOT operate within a transaction
func (r *CustomerRepository) FindByID(ctx context.Context, id string) (*domain.Customer, error) {
	return r.find(ctx, id, false)
}

// FindByID will locate a customer in the database using the provided ID and return
// a domain reconstituted type. Ooperates within a transaction block
func (r *CustomerRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.Customer, error) {
	return r.find(ctx, id, true)
}

//-----------------------------------------------------------------------------

// find is the helper method that will return the database record. It can operate with or without
// locking the row
func (r *CustomerRepository) find(ctx context.Context, id string, forUpdate bool) (*domain.Customer, error) {

	// select selectStmt
	selectStmt := `
	SELECT id,name,sms_number,enabled 
	FROM customers 
	WHERE id = $1
	`

	// if operating in a transaction, adds the row locking line
	if forUpdate {
		selectStmt += `
		FOR UPDATE
		`
	}

	// extract the transaction object
	exec := executorFrom(ctx, r.db)

	// variables that will retain data from the database
	var (
		customerID string
		name       string
		smsNumber  string
		enabled    bool
	)

	// query the database an store the returned values
	err := exec.QueryRow(ctx, selectStmt, id).Scan(&customerID, &name, &smsNumber, &enabled)
	if err != nil {
		switch {
		// didn't find the record in the database
		case errors.Is(err, pgx.ErrNoRows):
			return nil, domain.ErrCustomerNotFound
			// some other error
		default:
			return nil, fmt.Errorf("finding customer %q: %w", id, err)
		}
	}

	// create the domain type from the variables values
	return domain.ReconstituteCustomer(customerID, name, smsNumber, enabled), nil
}
