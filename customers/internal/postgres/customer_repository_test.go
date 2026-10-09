package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	"github.com/dupakarovsky/eda-storefront/customers/internal/postgres"
	foundationconfig "github.com/dupakarovsky/eda-storefront/foundation/config"
	foundationpostgres "github.com/dupakarovsky/eda-storefront/foundation/postgres"
	"github.com/joho/godotenv"
)

// newTestRepo will return a new CustomerRepository type for the
// duration of the test
func newTestRepo(t *testing.T) *postgres.CustomerRepository {

	t.Helper()

	// load the variables  from the .env file
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load customers/.env: %v", err)
	}

	// create a Config struct
	cfg, err := foundationconfig.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	dbCfg := foundationpostgres.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		Schema:          cfg.DB.Schema,
		SSLMode:         foundationpostgres.SSLMode(cfg.DB.SSLMode),
		MaxConns:        int32(cfg.DB.MaxConns),
		MinConns:        int32(cfg.DB.MinConns),
		MaxConnLifetime: cfg.DB.MaxConnLifetime,
		MaxConnIdleTime: cfg.DB.MaxConnIdleTime,
	}

	// creawte a connection pool that closes when the test is done
	pool, err := foundationpostgres.NewPool(context.Background(), dbCfg)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	// return the repository concrete type using the connection pool
	return postgres.NewCusomerRepository(pool)
}

// uniqueID defines a unique ID for running the test
func uniqueID(tag string) string {
	return tag + "-" + time.Now().Format("20060102150405.000000")
}

// seedCustomer adds in a customer into the database to running the test
func seedCustomer(t *testing.T, repo *postgres.CustomerRepository, tag string) *domain.Customer {
	t.Helper()

	// create a new customer
	customer, _, err := domain.NewCustomer(uniqueID(tag), "Alice Smith", "+5531999990000")
	if err != nil {
		t.Fatalf("failed to construct customer: %v", err)
	}

	// then save it to the repository
	if err := repo.Save(t.Context(), customer); err != nil {
		t.Fatalf("failed to save customer: %v", err)
	}

	return customer
}

func TestCustomerRepository_SaveAndFindByID_RoundTrips(t *testing.T) {
	// create a repository
	repo := newTestRepo(t)
	ctx := t.Context()

	// seed the database with a customer
	seeded := seedCustomer(t, repo, "rountrip")

	// call FindByID: try locating the customer using the ID
	found, err := repo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}

	// verify if the retruned Customer matches the expected values
	if found.ID != seeded.ID || found.Name != "Alice Smith" || found.SmsNumber != "+5531999990000" {
		t.Errorf("customer fields not restored as expected: %+v", found)
	}
	// verify if the customer is Enabled by default
	if !found.Enabled {
		t.Errorf("expected customer to be enabled")
	}
}

func TestCustomerRepository_FindByID_ReturnsNotFoundForUnknownID(t *testing.T) {
	// create a repository
	repo := newTestRepo(t)

	// try locating the unsaved Customer
	_, err := repo.FindByID(t.Context(), uniqueID("never-saved"))
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}

}

func TestCustomerRepository_Save_UpdatesExistingCustomer(t *testing.T) {
	// new reposiotyr
	repo := newTestRepo(t)
	ctx := t.Context()

	// seed the database with a customer
	seeded := seedCustomer(t, repo, "upsert")

	// call the ChangeSmsNumber method to update the sms number
	if _, err := seeded.ChangeSmsNumber("+5531888880000"); err != nil {
		t.Fatalf("failed to change sms number: %v", err)
	}

	// calls the Disable method; expect to succeed and set
	// the Customer to Diabled
	if _, err := seeded.Disable(); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}
	// update the customer in the repository
	if err := repo.Save(ctx, seeded); err != nil {
		t.Fatalf("failed to save updated customer: %v", err)
	}

	// now locate that customer using the ID
	found, err := repo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}
	// verify the sms number was persisted
	if found.SmsNumber != "+5531888880000" {
		t.Errorf("expected updated sms number, got %s", found.SmsNumber)
	}
	// verify the customer is Enabled by default
	if found.Enabled {
		t.Errorf("expected customer to be disabled after update")
	}
}

func TestCustomerRepository_FindByIDForUpdate_ReturnsNotFoundForUnknownID(t *testing.T) {

	// new repository
	repo := newTestRepo(t)

	// active a transaction. try locate an unsaved customer
	err := repo.WithLock(t.Context(), func(txCtx context.Context) error {
		_, err := repo.FindByIDForUpdate(txCtx, uniqueID("never-saved"))
		return err
	})
	// verify the error returned is ErrCustomerNotFound
	if !errors.Is(err, domain.ErrCustomerNotFound) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrCustomerNotFound, err)
	}
}

func TestCustomerRepository_WithLock_CommitsOnSuccess(t *testing.T) {

	// new repository
	repo := newTestRepo(t)
	ctx := t.Context()

	// seed the database with a customer
	seeded := seedCustomer(t, repo, "commit")

	// start a transaction
	err := repo.WithLock(ctx, func(txCtx context.Context) error {
		// locate the seeded customer in the databse
		customer, err := repo.FindByIDForUpdate(txCtx, seeded.ID)
		if err != nil {
			return err
		}
		// now update the sms number;
		if _, err := customer.ChangeSmsNumber("+5531888880000"); err != nil {
			return err
		}
		// finaly persist the change to the database
		return repo.Save(txCtx, customer)
	})
	// expect the transaction to succeed
	if err != nil {
		t.Fatalf("expected WithLock to succeed, got error: %v", err)
	}

	// locate the same customer in the database
	found, err := repo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}
	// verify the sms number change has persisted
	if found.SmsNumber != "+5531888880000" {
		t.Errorf("expected committed sms number, got %s", found.SmsNumber)
	}
}

func TestCustomerRepository_WithLock_RollsBackOnCallbackError(t *testing.T) {
	// new repository
	repo := newTestRepo(t)
	ctx := t.Context()

	// seed the database with a customer
	seeded := seedCustomer(t, repo, "rollback")
	// sentinel errorf for the test
	errBoom := errors.New("boom")

	// start a transaction
	err := repo.WithLock(ctx, func(txCtx context.Context) error {
		// try located the seeded customer by it's id
		customer, err := repo.FindByIDForUpdate(txCtx, seeded.ID)
		if err != nil {
			return err
		}
		// update the customer sms number
		if _, err := customer.ChangeSmsNumber("+5531888880000"); err != nil {
			return err
		}
		// persist the change
		if err := repo.Save(txCtx, customer); err != nil {
			return err
		}

		// force and error to happen
		return errBoom

		// transaction will now rollback
	})

	// check the error returned is the sentinel error
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected callback error to be returned, got: %v", err)
	}

	// locate the seeded customer in the database again
	found, err := repo.FindByID(ctx, seeded.ID)
	if err != nil {
		t.Fatalf("expected FindByID to succeed, got error: %v", err)
	}

	// verify the number was not updated
	if found.SmsNumber != "+5531999990000" {
		t.Errorf("expected write to be rolled back, got sms number %s", found.SmsNumber)
	}
}

func TestCustomerRepository_WithLock_RollsBackAfterCallerContextExpires(t *testing.T) {
	// new repositoyr
	repo := newTestRepo(t)

	// seed the reposiotry with a customer
	seeded := seedCustomer(t, repo, "rollback-expired")
	// sentinel error to happen
	errBoom := errors.New("boom")

	// create a timed context to run the command
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// start a transaction
	err := repo.WithLock(ctx, func(txCtx context.Context) error {
		// try locating the seeded customer in the database
		if _, err := repo.FindByIDForUpdate(txCtx, seeded.ID); err != nil {
			return err
		}
		// cancel the outside context manually
		cancel()

		// trigger an error manually; func is operating inside txCtx and not
		// ctx, so errBoom should be retruned insted of a context error
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected the callback error to survive an already-expired context, got: %v", err)
	}

	// from here the transaction has rolledback
	// ----------------------------------------

	// create a new context for 2 seconds
	lockCtx, lockCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer lockCancel()

	// start a new transaction
	err = repo.WithLock(lockCtx, func(txCtx context.Context) error {
		// locate the seeded customer in the database; should succeed
		_, err := repo.FindByIDForUpdate(txCtx, seeded.ID)
		return err
	})
	//
	if err != nil {
		t.Errorf("expected the row lock to be released after rollback, got: %v", err)
	}
}

func TestCustomerRepository_WithLock_SerializesConcurrentAccess(t *testing.T) {

	// new repository
	repo := newTestRepo(t)
	ctx := t.Context()

	// seed a customer into the database
	seeded := seedCustomer(t, repo, "lock")

	// duration for holding a lock
	const holdDuration = 300 * time.Millisecond

	// channel for blocking the main thread
	lockAcquired := make(chan struct{})

	// sync to run once
	var signalOnce sync.Once
	// closes the blocking channel and never runs again
	signal := func() { signalOnce.Do(func() { close(lockAcquired) }) }

	var lockedAt time.Time
	var returnedAt time.Time
	var seenBySecond string
	var wg sync.WaitGroup

	// create slots for 2 goroutines
	wg.Add(2)

	// #1 -- gets the lock first
	go func() {
		defer wg.Done()
		// will close the blocking channel when completes
		// and allow #2 to proceed
		defer signal()

		// start a transaction
		err := repo.WithLock(ctx, func(txCtx context.Context) error {
			// locate the customer in the databse
			customer, err := repo.FindByIDForUpdate(txCtx, seeded.ID)
			if err != nil {
				return err
			}

			// record the time the lock as accquired
			lockedAt = time.Now()

			// close the cannel now. this will alow #2 to proceed with it's
			// code. However #2 will have to wait until #1 releases the
			// transaction lock
			signal()

			// sleep for 500ms
			time.Sleep(holdDuration)

			// update the customer sms number
			if _, err := customer.ChangeSmsNumber("+5531888880000"); err != nil {
				return err
			}
			// save to repository
			return repo.Save(txCtx, customer)
		})
		if err != nil {
			t.Errorf("goroutine A: WithLock failed: %v", err)
		}
	}()

	// #2 -- waits for #1 to release the row lock for it to proceed
	go func() {
		defer wg.Done()

		// goroutine is blocked until #1 gets unblocks by callling
		// signal()
		<-lockAcquired

		// try to starts a transaction. It'll have to wait until
		// #1 releases the row lock
		err := repo.WithLock(ctx, func(txCtx context.Context) error {
			// locate the customer
			customer, err := repo.FindByIDForUpdate(txCtx, seeded.ID)
			// record the time it completed
			returnedAt = time.Now()
			if err != nil {
				return err
			}
			// record the sms number has retrieved from the database:
			// should be the number #1 updated to
			seenBySecond = customer.SmsNumber
			return nil
		})
		if err != nil {
			t.Errorf("goroutine B: WithLock failed: %v", err)
		}
	}()

	// wait until both goroutines to complete before exiting
	wg.Wait()

	// check the held duration
	if held := returnedAt.Sub(lockedAt); held < holdDuration {
		t.Errorf("expected goroutine B to be blocked until A committed (at least %s after A took the lock), but it returned after only %s", holdDuration, held)
	}

	// #2 should see the updated sms number
	if seenBySecond != "+5531888880000" {
		t.Errorf("expected goroutine B to read A's committed value, got %q", seenBySecond)
	}
}
