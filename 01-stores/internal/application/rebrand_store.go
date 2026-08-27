package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
)

// RebrandStore defines a struct type that is able to cummunicate
// with the repository and publisher to persist a Store's rebrand action
type RebrandStore struct {
	storeRepo domain.StoreRepository
	publisher *foundationnats.Publisher
	logger    *slog.Logger
}

// NewRebrandStore will instantaite a new NewRebrandStore
func NewRebrandStore(storeRepo domain.StoreRepository, publisher *foundationnats.Publisher, logger *slog.Logger) *RebrandStore {
	return &RebrandStore{
		storeRepo: storeRepo,
		publisher: publisher,
		logger:    logger,
	}
}

// Execute will locate a store that needs rebranding, update it to the repository than
// publish the Rebrand event to NATS
func (uc *RebrandStore) Execute(ctx context.Context, storeId, name string) (*domain.Store, error) {

	// event and store will capture the values returned by the FindByIDForUpdate and RebrandStore
	// methods
	var event domain.Event
	var store *domain.Store

	// WithLock will start a new transaction where the FindByIDForUpdate
	// method will execute, followed by Rebrand and Save, to presist the
	// changes to the database.
	err := uc.storeRepo.WithLock(ctx, func(txCtx context.Context) error {
		var err error

		// locate the store by it's id in the repository
		store, err = uc.storeRepo.FindByIDForUpdate(txCtx, storeId)
		if err != nil {
			switch {
			// store not found
			case errors.Is(err, domain.ErrStoreNotFound):
				return fmt.Errorf("finding store: %w", err)
				// unexpected error
			default:
				uc.logger.Error("unknown error finding store: %w", "error", err)
				return err
			}
		}

		// store located. Rebrand the store to emit the event
		// -------------------------------------------------

		// Rebrand the store to emit the event
		event, err = store.Rebrand(name)
		if err != nil {
			return fmt.Errorf("rebrand event: %w", err)
		}

		// event emited. update the store in the repository
		// ------------------------------------------------

		// Save will persiste the new name to the database via UPSERT command
		if err = uc.storeRepo.Save(txCtx, store); err != nil {
			return fmt.Errorf("saving store: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Transaction successful. Store was updated. Publish the events on NATS
	// --------------------------------------------------------------------

	//INFO:: If publishDomainEvent fails, the store remains created in the database.
	//Only the event was not pubslihed. This is a bit unusual, but right now it's what
	//we can do until we use a more robusts solution that uses Transaction outbox.

	if err = publishDomainEvent(ctx, uc.publisher, uc.logger, event); err != nil {
		return store, err
	}

	return store, nil
}
