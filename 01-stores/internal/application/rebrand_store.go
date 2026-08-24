package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	foundationnats "github.com/dupakarovsky/eda-storefront/foundation/nats"
	"github.com/dupakarovsky/eda-storefront/stores/internal/domain"
	storesevents "github.com/dupakarovsky/eda-storefront/stores/internal/events"
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

// Execute will connect to the database to persist the Store's name change, than
func (uc *RebrandStore) Execute(ctx context.Context, storeId, name string) (*domain.Store, error) {

	// locate the store by it's id in the repository
	store, err := uc.storeRepo.FindByID(ctx, storeId)
	if err != nil {
		switch {
		// store not found
		case errors.Is(err, domain.ErrStoreNotFound):
			return nil, fmt.Errorf("finding store: %w", err)
			// unexpected error
		default:
			uc.logger.Error("unknown error finding store: %w", "error", err)
			return nil, err
		}
	}

	// store located. Rebrand the store to emit the event
	// -------------------------------------------------

	// Rebrand the store to emit the event
	event, err := store.Rebrand(name)
	if err != nil {
		return nil, fmt.Errorf("rebrand event: %w", err)
	}

	// event emited. update the store in the repository
	// ------------------------------------------------

	// Save will persiste the new name to the database via UPSERT command
	if err = uc.storeRepo.Save(ctx, store); err != nil {
		return nil, fmt.Errorf("saving store: %w", err)
	}

	// store was updated. Publish the events on NATS
	// ------------------------------------------------

	// prepare a new envelope for transporting the event
	envelope, err := storesevents.NewEnvelope(event)
	if err != nil {
		return nil, fmt.Errorf("building envelope: %w", err)
	}

	// conver the envelope into JSON blob
	payload, err := envelope.Marshal()

	// create a subject to publish to the NATS stream
	subject, err := storesevents.SubjectFor(event)
	if err != nil {
	}

	// publish the event to nats using the retry funtion
	ack, err := publishWithRetry(ctx, uc.publisher, uc.logger, subject, payload)
	if err != nil {
		uc.logger.Error("event publish exhausted retries; store saved but event not delivered",
			"aggregate_id", event.AggregateID(),
			"event_name", event.EventName(),
			"subject", subject,
			"error", err,
		)

		//INFO:: If publisheWithRetry fails, the store remains created in the database.
		//Only the event was not pubslihed. This is a bit unusual, but right now it's what
		//we can do until we use a more robusts solution that uses Transaction outbox.

		return store, fmt.Errorf("publishing event: %w", err)
	}

	// event published. emit a log
	uc.logger.Info("event puslished", "subject", subject, "stream", ack.Stream, "seq", ack.Sequence)

	return store, nil
}
