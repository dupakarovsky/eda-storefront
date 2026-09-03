package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

func TestRebrandStore_Succeeds(t *testing.T) {

	// instantiate the module and the context for the test
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// unique ID for this test
	id := "rebrand-test-" + time.Now().Format("20060102150405.000000")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Original Name", "1 Original Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// call the RebrandStoreUC; expected to succeed;
	store, err := mod.RebrandStoreUC.Execute(ctx, id, "New Name")
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	// expected the name to have changed
	if store.Name != "New Name" {
		t.Errorf("expected store name %q, got %q", "New Name", store.Name)
	}
}

func TestRebrandStore_RejectsBlankName(t *testing.T) {

	// instantiate the module and the context for the test
	mod := newTestModule("application_test.log", t)
	ctx := t.Context()

	// unique ID for this test
	id := "rebrand-blank-test-" + time.Now().Format("20060102150405.000000")

	// create a new store
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Original Name", "1 Original Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// call the RebrandStoreUC with blank name; expected to fail;
	_, err = mod.RebrandStoreUC.Execute(ctx, id, "")
	if nil == err {
		t.Fatal("expected Execute to fail on blank name, got nil error")
	}

	// expected the name to have changed
	if !errors.Is(err, domain.ErrStoreNameIsBlank) {
		t.Errorf("expected error to wrap %v, got: %v", domain.ErrStoreNameIsBlank, err)
	}
}

// TestStoreRepository_WithLock_SerializesConcurrentAccess will test the transaction
// locking for the StoreRebrand operation
func TestStoreRepository_WithLock_SerializesConcurrentAccess(t *testing.T) {

	// initialize a new test module
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	// define a unique ID for the test
	id := uniqueID("lock-test")
	// create a new store with that id
	_, err := mod.CreateStoreUC.Execute(ctx, id, "Lock Test Grocer", "1 Lock Ave")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// time that transaction will remain locked lock
	const holdDuration = 300 * time.Millisecond

	// defines a chanel that will be closed when the transaction starts, allowing
	// the next caller to begin it's operation and be locked out until the first
	// transaction terminates.
	lockAcquired := make(chan struct{})

	// waitedFor will record the time it took for the Waiting Goroutine to
	// complete it's work.
	var waitedFor time.Duration
	var wg sync.WaitGroup

	// spawn 2 goroutines
	wg.Add(2)

	// Active Goroutine
	go func() {
		defer wg.Done()

		// call the WithLock method will start a new transaction.
		err := mod.StoreRepo.WithLock(ctx, func(txCtx context.Context) error {

			// locate a store that needs it's name changed
			store, err := mod.StoreRepo.FindByIDForUpdate(txCtx, id)
			if err != nil {
				return err
			}

			// lock has been accquired
			close(lockAcquired)

			// hold the lock for 300ms
			time.Sleep(holdDuration)

			// proceed to Rebrand the store within a transaction lock
			if _, err := store.Rebrand("Renamed by A"); err != nil {
				return err
			}

			// save to the repository within the transaction lock
			return mod.StoreRepo.Save(txCtx, store)
		})
		if err != nil {
			t.Errorf("goroutine A: WithLock failed: %v", err)
		}
	}()

	// Waiting goroutine
	go func() {
		defer wg.Done()

		// this goroutine will keep receiving from an unbeffered empty
		// channel until it's closed. The moment this channel is closed the
		// Active goroutine will be in transaction lock
		<-lockAcquired

		// start recording the time
		start := time.Now()

		// now start it's own transaction
		err := mod.StoreRepo.WithLock(ctx, func(txCtx context.Context) error {

			// try locating a store with the same id. This row will already be in
			// a transaction lock. It'll need to wait until the Active goroutine
			// releases the lock.
			_, err := mod.StoreRepo.FindByIDForUpdate(txCtx, id)
			// once it's finished it's operation we record the time it took
			//to complete
			waitedFor = time.Since(start)

			return err
		})
		if err != nil {
			t.Errorf("goroutine B: WithLock failed: %v", err)
		}
	}()

	// wait for all goroutines to complete before exiting
	wg.Wait()

	// evaluate the time it took for the Waiting Roroutine to finish
	// its task. It should take at least 300 ms
	if waitedFor < holdDuration {
		t.Errorf("expected goroutine B to block for at least %s while A held the lock, but it only waited %s", holdDuration, waitedFor)
	}
}
